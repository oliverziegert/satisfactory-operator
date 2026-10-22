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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

var _ = Describe("SatisfactoryServer Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		satisfactoryserver := &satisfactoryv1alpha1.SatisfactoryServer{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind SatisfactoryServer")
			err := k8sClient.Get(ctx, typeNamespacedName, satisfactoryserver)
			if err != nil && errors.IsNotFound(err) {
				resource := &satisfactoryv1alpha1.SatisfactoryServer{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &satisfactoryv1alpha1.SatisfactoryServer{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SatisfactoryServer")
			// No controller runs in envtest to clear the finalizer; a leftover server skews ClusterPolicy counts.
			resource.Finalizers = nil
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &SatisfactoryServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("When reconciling Service ports", func() {
		ctx := context.Background()
		var (
			key        types.NamespacedName
			reconciler *SatisfactoryServerReconciler
		)

		getServer := func() *satisfactoryv1alpha1.SatisfactoryServer {
			server := &satisfactoryv1alpha1.SatisfactoryServer{}
			Expect(k8sClient.Get(ctx, key, server)).To(Succeed())
			return server
		}
		getService := func(name string) *corev1.Service {
			svc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: key.Namespace}, svc)).To(Succeed())
			return svc
		}
		nodePorts := func(svc *corev1.Service) map[string]int32 {
			m := map[string]int32{}
			for _, p := range svc.Spec.Ports {
				m[p.Name] = p.NodePort
			}
			return m
		}
		ports := func(svc *corev1.Service) map[string]int32 {
			m := map[string]int32{}
			for _, p := range svc.Spec.Ports {
				m[p.Name] = p.Port
			}
			return m
		}
		reconcileOnce := func() error {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			return err
		}
		updateSpec := func(mutate func(*satisfactoryv1alpha1.SatisfactoryServerSpec)) {
			server := getServer()
			mutate(&server.Spec)
			Expect(k8sClient.Update(ctx, server)).To(Succeed())
		}
		createServer := func(name string, mutate func(*satisfactoryv1alpha1.SatisfactoryServerSpec)) {
			key = types.NamespacedName{Name: name, Namespace: "default"}
			server := &satisfactoryv1alpha1.SatisfactoryServer{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
			mutate(&server.Spec)
			Expect(k8sClient.Create(ctx, server)).To(Succeed())
			// The first reconcile only adds the finalizer.
			Expect(reconcileOnce()).To(Succeed())
		}

		BeforeEach(func() {
			reconciler = &SatisfactoryServerReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: events.NewFakeRecorder(100),
			}
		})

		AfterEach(func() {
			server := getServer()
			server.Finalizers = nil
			Expect(k8sClient.Update(ctx, server)).To(Succeed())
			Expect(k8sClient.Delete(ctx, server)).To(Succeed())
			// No garbage collector runs in envtest; free the pinned node ports for the next case.
			for _, name := range []string{key.Name, key.Name + "-headless"} {
				svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: key.Namespace}}
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, svc))).To(Succeed())
			}
		})

		It("updates both Services on a gamePort change and keeps unchanged node ports", func() {
			createServer("svc-port-change", func(*satisfactoryv1alpha1.SatisfactoryServerSpec) {})
			Expect(reconcileOnce()).To(Succeed())
			before := nodePorts(getService(key.Name))
			Expect(before["messaging"]).NotTo(BeZero())

			updateSpec(func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) { s.GamePort = 7778 })
			Expect(reconcileOnce()).To(Succeed())

			external := getService(key.Name)
			Expect(ports(external)).To(Equal(map[string]int32{"game-tcp": 7778, "game-udp": 7778, "messaging": 8888}))
			Expect(nodePorts(external)["messaging"]).To(Equal(before["messaging"]))
			Expect(ports(getService(key.Name + "-headless"))).To(
				Equal(map[string]int32{"game-tcp": 7778, "game-udp": 7778, "messaging": 8888}))
		})

		It("pins node ports when switching from LoadBalancer to NodePort", func() {
			createServer("svc-type-switch", func(*satisfactoryv1alpha1.SatisfactoryServerSpec) {})
			Expect(reconcileOnce()).To(Succeed())

			updateSpec(func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) {
				s.ServiceType = corev1.ServiceTypeNodePort
				s.GamePort = 30777
				s.MessagingPort = 30888
			})
			Expect(reconcileOnce()).To(Succeed())

			external := getService(key.Name)
			Expect(external.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
			want := map[string]int32{"game-tcp": 30777, "game-udp": 30777, "messaging": 30888}
			Expect(ports(external)).To(Equal(want))
			Expect(nodePorts(external)).To(Equal(want))
		})

		It("reports a rejected NodePort Service as Degraded and recovers", func() {
			createServer("svc-invalid", func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) {
				s.ServiceType = corev1.ServiceTypeNodePort
			})
			err := reconcileOnce()
			Expect(errors.IsInvalid(err)).To(BeTrue(), "got %v", err)

			server := getServer()
			degraded := meta.FindStatusCondition(server.Status.Conditions, satisfactoryv1alpha1.SatisfactoryServerConditionTypeDegraded)
			Expect(degraded).NotTo(BeNil())
			Expect(degraded.Status).To(Equal(metav1.ConditionTrue))
			Expect(degraded.Reason).To(Equal("ServiceInvalid"))
			Expect(degraded.Message).To(ContainSubstring("valid range"))
			Expect(meta.IsStatusConditionFalse(server.Status.Conditions, satisfactoryv1alpha1.SatisfactoryServerConditionTypeReady)).To(BeTrue())
			Expect(server.Status.Phase).To(Equal(satisfactoryv1alpha1.SatisfactoryServerPhaseError))
			Expect(reconciler.Recorder.(*events.FakeRecorder).Events).To(Receive(ContainSubstring("ServiceInvalid")))

			updateSpec(func(s *satisfactoryv1alpha1.SatisfactoryServerSpec) {
				s.GamePort = 30777
				s.MessagingPort = 30888
			})
			Expect(reconcileOnce()).To(Succeed())

			degraded = meta.FindStatusCondition(getServer().Status.Conditions, satisfactoryv1alpha1.SatisfactoryServerConditionTypeDegraded)
			Expect(degraded.Reason).NotTo(Equal("ServiceInvalid"))
			Expect(nodePorts(getService(key.Name))).To(
				Equal(map[string]int32{"game-tcp": 30777, "game-udp": 30777, "messaging": 30888}))
		})
	})
})
