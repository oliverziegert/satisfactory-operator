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
			Client:     k8sClient,
			Scheme:     k8sClient.Scheme(),
			GameAPIURL: func(*satisfactoryv1alpha1.SatisfactoryServer) string { return ts.URL },
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

	It("backs up on schedule against a reachable, authenticated server", func() {
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

		create(&satisfactoryv1alpha1.SaveGame{
			ObjectMeta: metav1.ObjectMeta{Name: "sg-hourly", Namespace: "default"},
			Spec:       satisfactoryv1alpha1.SaveGameSpec{ServerRef: "sg-srv", SaveName: "save", Schedule: "0 * * * *"},
		})

		res, sg := reconcileAndGet("sg-hourly")
		Expect(api.saved).To(HaveLen(1))
		Expect(sg.Status.Phase).To(Equal(satisfactoryv1alpha1.SaveGamePhaseActive))
		Expect(sg.Status.LastBackupTime).NotTo(BeNil())
		Expect(sg.Status.NextBackupTime).NotTo(BeNil())
		Expect(sg.Status.RetainedSaveCount).To(Equal(int32(1)))
		Expect(res.RequeueAfter).To(BeNumerically("<=", time.Hour))

		By("not saving again before the next tick")
		_, sg = reconcileAndGet("sg-hourly")
		Expect(api.saved).To(HaveLen(1))
		Expect(sg.Status.Phase).To(Equal(satisfactoryv1alpha1.SaveGamePhaseActive))
	})
})
