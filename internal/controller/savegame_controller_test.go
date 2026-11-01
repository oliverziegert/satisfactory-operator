/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
	"github.com/oliverziegert/satisfactory-operator/internal/conditions"
)

var _ = Describe("SaveGame Controller", func() {
	ctx := context.Background()

	var (
		api *fakeGameAPI
		rec *SaveGameReconciler
	)

	BeforeEach(func() {
		api = &fakeGameAPI{}
		ts := httptest.NewTLSServer(api.handler())
		DeferCleanup(ts.Close)
		rec = &SaveGameReconciler{
			Client:      k8sClient,
			Scheme:      k8sClient.Scheme(),
			BackupImage: "operator:test",
			GameAPIURL:  func(*satisfactoryv1alpha1.SatisfactoryServer) string { return ts.URL },
		}
	})

	create := func(obj client.Object) {
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed()) })
	}

	reconcileAndGet := func(name string) (reconcile.Result, *satisfactoryv1alpha1.SaveGame) {
		key := types.NamespacedName{Name: name, Namespace: "default"}
		res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		sg := &satisfactoryv1alpha1.SaveGame{}
		Expect(k8sClient.Get(ctx, key, sg)).To(Succeed())
		return res, sg
	}

	// reachableServer creates the SatisfactoryServer sg-srv with ServerReachable=True.
	reachableServer := func() {
		create(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "sg-srv-admin", Namespace: "default"},
			Data:       map[string][]byte{"password": []byte("pass")},
		})
		server := &satisfactoryv1alpha1.SatisfactoryServer{
			ObjectMeta: metav1.ObjectMeta{Name: "sg-srv", Namespace: "default"},
			Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
				AdminPasswordSecretRef: &corev1.LocalObjectReference{Name: "sg-srv-admin"},
			},
		}
		create(server)
		// The token cache Secret is owned by the server, but envtest has no garbage collector.
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: authSecretNameFor("sg-srv"), Namespace: "default"},
			}))).To(Succeed())
		})
		conditions.SetSatisfactoryServer(server, satisfactoryv1alpha1.SatisfactoryServerConditionTypeServerReachable,
			metav1.ConditionTrue, "Reachable", "API reachable")
		Expect(k8sClient.Status().Update(ctx, server)).To(Succeed())

	}

	jobs := func() []batchv1.Job {
		list := &batchv1.JobList{}
		Expect(k8sClient.List(ctx, list, client.InNamespace("default"))).To(Succeed())
		return list.Items
	}

	DescribeTable("validates spec.destinations",
		func(dests []satisfactoryv1alpha1.SaveGameDestination, wantErr string) {
			sg := &satisfactoryv1alpha1.SaveGame{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "sg-dest-", Namespace: "default"},
				Spec:       satisfactoryv1alpha1.SaveGameSpec{ServerRef: "srv", SaveName: "save", Destinations: dests},
			}
			err := k8sClient.Create(ctx, sg)
			if wantErr == "" {
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Delete(ctx, sg)).To(Succeed())
				return
			}
			Expect(err).To(MatchError(ContainSubstring(wantErr)))
		},
		Entry("valid server destination",
			[]satisfactoryv1alpha1.SaveGameDestination{{Name: "local", Server: &satisfactoryv1alpha1.ServerDestination{}}}, ""),
		Entry("no type", []satisfactoryv1alpha1.SaveGameDestination{{Name: "local"}},
			"exactly one destination type must be set"),
		Entry("two server destinations", []satisfactoryv1alpha1.SaveGameDestination{
			{Name: "a", Server: &satisfactoryv1alpha1.ServerDestination{}},
			{Name: "b", Server: &satisfactoryv1alpha1.ServerDestination{}},
		}, "at most one server destination is allowed"),
		Entry("duplicate names", []satisfactoryv1alpha1.SaveGameDestination{
			{Name: "offsite", Server: &satisfactoryv1alpha1.ServerDestination{}},
			{Name: "offsite"},
		}, "Duplicate value"),
		Entry("name not a DNS label",
			[]satisfactoryv1alpha1.SaveGameDestination{{Name: "Off_Site", Server: &satisfactoryv1alpha1.ServerDestination{}}},
			"should match"),
	)

	It("marks the SaveGame Failed when the server does not exist", func() {
		create(&satisfactoryv1alpha1.SaveGame{
			ObjectMeta: metav1.ObjectMeta{Name: "sg-orphan", Namespace: "default"},
			Spec:       satisfactoryv1alpha1.SaveGameSpec{ServerRef: "nope", SaveName: "save"},
		})

		res, sg := reconcileAndGet("sg-orphan")
		Expect(res.RequeueAfter).To(Equal(saveGameRetryInterval))
		Expect(sg.Status.Phase).To(Equal(satisfactoryv1alpha1.SaveGamePhaseFailed))
		Expect(meta.FindStatusCondition(sg.Status.Conditions, satisfactoryv1alpha1.SaveGameConditionTypeReady).Reason).
			To(Equal("ServerNotFound"))
	})

	It("runs scheduled backups as Jobs and records their result", func() {
		reachableServer()
		create(&satisfactoryv1alpha1.SaveGame{
			ObjectMeta: metav1.ObjectMeta{Name: "sg-hourly", Namespace: "default"},
			Spec:       satisfactoryv1alpha1.SaveGameSpec{ServerRef: "sg-srv", SaveName: "save", Schedule: "0 * * * *"},
		})

		By("creating one Job when the backup is due")
		_, sg := reconcileAndGet("sg-hourly")
		Expect(jobs()).To(HaveLen(1))
		job := jobs()[0]
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &job,
				client.PropagationPolicy(metav1.DeletePropagationBackground)))).To(Succeed())
		})
		Expect(metav1.IsControlledBy(&job, sg)).To(BeTrue())
		Expect(api.saved).To(BeEmpty(), "the reconciler must not save itself")
		Expect(sg.Status.NextBackupTime).NotTo(BeNil())

		By("not creating a second Job while the first is running")
		reconcileAndGet("sg-hourly")
		Expect(jobs()).To(HaveLen(1))

		By("recording the result of the finished Job")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: job.Name + "-pod", Namespace: "default",
				Labels: map[string]string{batchv1.JobNameLabel: job.Name}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: backupContainerName, Image: "operator:test"}}},
		}
		create(pod)
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name: backupContainerName, Image: "operator:test", ImageID: "operator@sha256:0",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0,
				Message: `{"saveName":"save_x","saved":true,"size":42,"destinations":[{"name":"server","ok":true,"retained":4}]}`}},
		}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		now := metav1.Now()
		job.Status = batchv1.JobStatus{StartTime: &now, CompletionTime: &now, Succeeded: 1, Conditions: []batchv1.JobCondition{
			{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
			{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		}}
		Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())

		res, sg := reconcileAndGet("sg-hourly")
		Expect(sg.Status.Phase).To(Equal(satisfactoryv1alpha1.SaveGamePhaseActive))
		Expect(meta.IsStatusConditionTrue(sg.Status.Conditions, satisfactoryv1alpha1.SaveGameConditionTypeReady)).To(BeTrue())
		Expect(sg.Status.LastBackupTime).NotTo(BeNil())
		Expect(*sg.Status.BackupSize).To(Equal(int64(42)))
		Expect(sg.Status.RetainedSaveCount).To(Equal(int32(4)))
		Expect(sg.Status.Destinations).To(HaveLen(1))
		Expect(sg.Status.Destinations[0].LastSuccessTime).NotTo(BeNil())
		Expect(res.RequeueAfter).To(BeNumerically("<=", time.Hour))
		Expect(jobs()).To(HaveLen(1), "no new Job before the next tick")
	})

	It("reports BackupImageNotConfigured without creating a Job", func() {
		rec.BackupImage = ""
		create(&satisfactoryv1alpha1.SaveGame{
			ObjectMeta: metav1.ObjectMeta{Name: "sg-noimage", Namespace: "default"},
			Spec:       satisfactoryv1alpha1.SaveGameSpec{ServerRef: "nope", SaveName: "save"},
		})

		_, sg := reconcileAndGet("sg-noimage")
		Expect(meta.FindStatusCondition(sg.Status.Conditions, satisfactoryv1alpha1.SaveGameConditionTypeReady).Reason).
			To(Equal("BackupImageNotConfigured"))
		list := &batchv1.JobList{}
		Expect(k8sClient.List(ctx, list, client.InNamespace("default"))).To(Succeed())
		Expect(list.Items).To(BeEmpty())
	})
})
