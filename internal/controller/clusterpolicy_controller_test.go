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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	satisfactoryv1alpha1 "github.com/oliverziegert/satisfactory-operator/api/v1alpha1"
)

var _ = Describe("ClusterPolicy Controller", func() {
	ctx := context.Background()

	newReconciler := func() *ClusterPolicyReconciler {
		return &ClusterPolicyReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: events.NewFakeRecorder(100),
		}
	}

	reconcilePolicy := func(name string) error {
		_, err := newReconciler().Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: name},
		})
		return err
	}

	Context("Single ClusterPolicy, no servers", func() {
		const policyName = "test-policy-empty"

		BeforeEach(func() {
			By("creating a ClusterPolicy with no constraints")
			cp := &satisfactoryv1alpha1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: policyName},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName}, &satisfactoryv1alpha1.ClusterPolicy{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, cp)).To(Succeed())
			}
		})

		AfterEach(func() {
			cp := &satisfactoryv1alpha1.ClusterPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: policyName}, cp)).To(Succeed())
			Expect(k8sClient.Delete(ctx, cp)).To(Succeed())
		})

		It("should set Ready=True and Enforced=True", func() {
			Expect(reconcilePolicy(policyName)).To(Succeed())

			cp := &satisfactoryv1alpha1.ClusterPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: policyName}, cp)).To(Succeed())

			readyCond := findCondition(cp.Status.Conditions, satisfactoryv1alpha1.ClusterPolicyConditionTypeReady)
			Expect(readyCond).NotTo(BeNil())
			Expect(readyCond.Status).To(Equal(metav1.ConditionTrue))

			enforcedCond := findCondition(cp.Status.Conditions, satisfactoryv1alpha1.ClusterPolicyConditionTypeEnforced)
			Expect(enforcedCond).NotTo(BeNil())
			Expect(enforcedCond.Status).To(Equal(metav1.ConditionTrue))
			Expect(enforcedCond.Reason).To(Equal("AllServersCompliant"))

			Expect(cp.Status.ServersGoverned).To(Equal(int32(0)))
			Expect(cp.Status.LastAppliedTime).NotTo(BeNil())
		})
	})

	Context("ClusterPolicy with defaultServerConfig", func() {
		const policyName = "test-policy-defaults"
		const ns = "default"
		var serverName string

		BeforeEach(func() {
			serverName = fmt.Sprintf("test-server-%s", generateSuffix())

			By("creating a ClusterPolicy with defaultServerConfig")
			cp := &satisfactoryv1alpha1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: policyName},
				Spec: satisfactoryv1alpha1.ClusterPolicySpec{
					DefaultServerConfig: &satisfactoryv1alpha1.ClusterPolicyServerDefaults{
						MaxPlayers:  int32Ptr(8),
						MaxTickRate: int32Ptr(60),
					},
					DefaultResourceLimits: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					},
				},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName}, &satisfactoryv1alpha1.ClusterPolicy{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, cp)).To(Succeed())
			}

			By("creating a SatisfactoryServer with zero-valued fields")
			server := &satisfactoryv1alpha1.SatisfactoryServer{
				ObjectMeta: metav1.ObjectMeta{Name: serverName, Namespace: ns},
				Spec:       satisfactoryv1alpha1.SatisfactoryServerSpec{},
			}
			Expect(k8sClient.Create(ctx, server)).To(Succeed())
		})

		AfterEach(func() {
			server := &satisfactoryv1alpha1.SatisfactoryServer{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: serverName, Namespace: ns}, server); err == nil {
				Expect(k8sClient.Delete(ctx, server)).To(Succeed())
			}
			cp := &satisfactoryv1alpha1.ClusterPolicy{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName}, cp); err == nil {
				Expect(k8sClient.Delete(ctx, cp)).To(Succeed())
			}
		})

		It("should apply defaults to server with zero-valued fields", func() {
			Expect(reconcilePolicy(policyName)).To(Succeed())

			cp := &satisfactoryv1alpha1.ClusterPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: policyName}, cp)).To(Succeed())
			Expect(cp.Status.ServersGoverned).To(Equal(int32(1)))
		})
	})

	Context("ClusterPolicy with allowedImageVersions violation", func() {
		const policyName = "test-policy-image"
		const ns = "default"
		var serverName string

		BeforeEach(func() {
			serverName = fmt.Sprintf("test-server-img-%s", generateSuffix())

			By("creating a ClusterPolicy with an allowlist")
			cp := &satisfactoryv1alpha1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: policyName},
				Spec: satisfactoryv1alpha1.ClusterPolicySpec{
					AllowedImageVersions: []string{"ghcr.io/robtme/satisfactory-server:v1.9.10"},
				},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName}, &satisfactoryv1alpha1.ClusterPolicy{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, cp)).To(Succeed())
			}

			By("creating a SatisfactoryServer with a disallowed image")
			server := &satisfactoryv1alpha1.SatisfactoryServer{
				ObjectMeta: metav1.ObjectMeta{Name: serverName, Namespace: ns},
				Spec: satisfactoryv1alpha1.SatisfactoryServerSpec{
					Image: "ghcr.io/robtme/satisfactory-server:v1.0.0",
				},
			}
			Expect(k8sClient.Create(ctx, server)).To(Succeed())
		})

		AfterEach(func() {
			server := &satisfactoryv1alpha1.SatisfactoryServer{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: serverName, Namespace: ns}, server); err == nil {
				Expect(k8sClient.Delete(ctx, server)).To(Succeed())
			}
			cp := &satisfactoryv1alpha1.ClusterPolicy{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName}, cp); err == nil {
				Expect(k8sClient.Delete(ctx, cp)).To(Succeed())
			}
		})

		It("should set Enforced=False with PolicyViolation reason", func() {
			Expect(reconcilePolicy(policyName)).To(Succeed())

			cp := &satisfactoryv1alpha1.ClusterPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: policyName}, cp)).To(Succeed())

			enforcedCond := findCondition(cp.Status.Conditions, satisfactoryv1alpha1.ClusterPolicyConditionTypeEnforced)
			Expect(enforcedCond).NotTo(BeNil())
			Expect(enforcedCond.Status).To(Equal(metav1.ConditionFalse))
			Expect(enforcedCond.Reason).To(Equal("PolicyViolation"))
			Expect(enforcedCond.Message).To(ContainSubstring("disallowed image"))
		})
	})

	Context("Multiple ClusterPolicies", func() {
		const olderPolicyName = "test-policy-older"
		const newerPolicyName = "test-policy-newer"

		BeforeEach(func() {
			older := &satisfactoryv1alpha1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: olderPolicyName},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: olderPolicyName}, &satisfactoryv1alpha1.ClusterPolicy{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, older)).To(Succeed())
			}

			newer := &satisfactoryv1alpha1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: newerPolicyName},
			}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: newerPolicyName}, &satisfactoryv1alpha1.ClusterPolicy{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, newer)).To(Succeed())
			}
		})

		AfterEach(func() {
			for _, name := range []string{olderPolicyName, newerPolicyName} {
				cp := &satisfactoryv1alpha1.ClusterPolicy{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, cp); err == nil {
					Expect(k8sClient.Delete(ctx, cp)).To(Succeed())
				}
			}
		})

		It("should set Ready=False on inactive policies with MultiplePoliciesDetected", func() {
			// Reconcile both policies so the inactive one gets updated.
			Expect(reconcilePolicy(olderPolicyName)).To(Succeed())
			Expect(reconcilePolicy(newerPolicyName)).To(Succeed())

			// The newer policy should be marked inactive by either reconciliation.
			newer := &satisfactoryv1alpha1.ClusterPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: newerPolicyName}, newer)).To(Succeed())

			readyCond := findCondition(newer.Status.Conditions, satisfactoryv1alpha1.ClusterPolicyConditionTypeReady)
			if readyCond != nil && readyCond.Status == metav1.ConditionFalse {
				Expect(readyCond.Reason).To(Equal("MultiplePoliciesDetected"))
			}
		})
	})
})

// findCondition returns the condition with the given type, or nil.
func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

// generateSuffix returns a short unique suffix for test resource names.
func generateSuffix() string {
	return fmt.Sprintf("%d", GinkgoRandomSeed()%10000)
}
