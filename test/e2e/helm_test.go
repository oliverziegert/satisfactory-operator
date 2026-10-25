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

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/oliverziegert/satisfactory-operator/test/utils"
)

const (
	helmRelease = "satisfactory-operator"
	helmChart   = "dist/chart"
	crdGroup    = "satisfactory.oliverziegert.github.com"
)

var crdNames = []string{"clusterpolicies", "satisfactoryservers", "savegames"}

var _ = Describe("Helm chart", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	BeforeAll(func() {
		createNamespaces("games", "other")

		By("installing the chart in namespace mode with a ClusterPolicy")
		helmInstall(
			"--set", "watchNamespaces={games}",
			"--set", "clusterPolicy.enabled=true",
			"--set", "clusterPolicy.spec.defaultServerConfig.maxPlayers=7",
		)
	})

	AfterAll(func() {
		cleanupHelmInstall("games", "other")
	})

	It("should run the manager with the watch namespaces", func() {
		out := mustRun("kubectl", "get", "deploy", "-n", namespace, "-l", "control-plane=controller-manager",
			"-o", "jsonpath={.items[0].spec.template.spec.containers[0].args}")
		Expect(out).To(ContainSubstring("--watch-namespaces=games"))
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "pods", "-n", namespace,
				"-l", "control-plane=controller-manager", "-o", "jsonpath={.items[*].status.phase}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("Running"))
		}).Should(Succeed())
	})

	It("should create the ClusterPolicy in the same install", func() {
		out := mustRun("kubectl", "get", "clusterpolicy", "default",
			"-o", "jsonpath={.spec.defaultServerConfig.maxPlayers}")
		Expect(out).To(Equal("7"))
	})

	It("should limit the manager's permissions to the watch namespaces", func() {
		sa := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccountName)
		Expect(canI(sa, "get", "secrets", "games")).To(BeTrue())
		Expect(canI(sa, "get", "secrets", "default")).To(BeFalse())
		Expect(canI(sa, "get", "secrets", "other")).To(BeFalse())
	})

	It("should reconcile a server in a watched namespace only", func() {
		createServer("games")
		createServer("other")

		Eventually(func(g Gomega) {
			_, err := utils.Run(exec.Command("kubectl", "get", "statefulset", "srv", "-n", "games"))
			g.Expect(err).NotTo(HaveOccurred())
		}).Should(Succeed())

		By("checking that the policy defaults were patched onto the watched server")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "satisfactoryserver", "srv", "-n", "games",
				"-o", "jsonpath={.spec.maxPlayers}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("7"))
		}).Should(Succeed())

		Consistently(func(g Gomega) {
			_, err := utils.Run(exec.Command("kubectl", "get", "statefulset", "srv", "-n", "other"))
			g.Expect(err).To(HaveOccurred())
		}, 10*time.Second).Should(Succeed())
	})
})

var _ = Describe("Migration from install.yaml", Ordered, func() {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	BeforeAll(func() {
		By("installing the operator from the kustomize manifests (install.yaml)")
		mustRun("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		createNamespaces("games")
		createServer("games")
		Eventually(func(g Gomega) {
			_, err := utils.Run(exec.Command("kubectl", "get", "pvc", "config-srv-0", "-n", "games"))
			g.Expect(err).NotTo(HaveOccurred())
		}).Should(Succeed())
	})

	AfterAll(func() {
		cleanupHelmInstall("games")
	})

	It("should keep the servers when following the README migration", func() {
		By("deleting the operator objects by kind and name, never the CRDs")
		mustRun("kubectl", "delete", "namespace", namespace)
		mustRun("kubectl", "delete", "clusterrolebinding",
			"satisfactory-operator-manager-rolebinding", "satisfactory-operator-metrics-auth-rolebinding")
		roles := make([]string, 0, 12)
		roles = append(roles, "satisfactory-operator-manager-role", "satisfactory-operator-metrics-auth-role",
			"satisfactory-operator-metrics-reader")
		for _, kind := range []string{"clusterpolicy", "satisfactoryserver", "savegame"} {
			for _, verb := range []string{"admin", "editor", "viewer"} {
				roles = append(roles, fmt.Sprintf("satisfactory-operator-%s-%s-role", kind, verb))
			}
		}
		mustRun("kubectl", append([]string{"delete", "clusterrole"}, roles...)...)

		By("handing the CRDs over to the Helm release")
		for _, name := range crdNames {
			crd := name + "." + crdGroup
			mustRun("kubectl", "label", "crd", crd, "app.kubernetes.io/managed-by=Helm", "--overwrite")
			mustRun("kubectl", "annotate", "crd", crd, "meta.helm.sh/release-name="+helmRelease,
				"meta.helm.sh/release-namespace="+namespace, "--overwrite")
		}

		helmInstall()

		By("checking that the server's StatefulSet and PVC survived")
		mustRun("kubectl", "get", "statefulset", "srv", "-n", "games")
		mustRun("kubectl", "get", "pvc", "config-srv-0", "-n", "games")

		By("checking that the new controller reconciles the server")
		mustRun("kubectl", "patch", "satisfactoryserver", "srv", "-n", "games", "--type=merge",
			"-p", `{"spec":{"maxPlayers":5}}`)
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "satisfactoryserver", "srv", "-n", "games",
				"-o", "jsonpath={.metadata.generation} {.status.conditions[0].observedGeneration}"))
			g.Expect(err).NotTo(HaveOccurred())
			fields := strings.Fields(out)
			g.Expect(fields).To(HaveLen(2))
			g.Expect(fields[1]).To(Equal(fields[0]))
		}).Should(Succeed())
	})
})

func mustRun(name string, args ...string) string {
	GinkgoHelper()
	out, err := utils.Run(exec.Command(name, args...))
	Expect(err).NotTo(HaveOccurred())
	return out
}

func helmInstall(args ...string) {
	GinkgoHelper()
	repo, tag, _ := strings.Cut(projectImage, ":")
	mustRun("helm", append([]string{"install", helmRelease, helmChart,
		"-n", namespace, "--create-namespace", "--wait", "--timeout", "3m",
		"--set", "manager.image.repository=" + repo, "--set", "manager.image.tag=" + tag}, args...)...)
}

func createNamespaces(names ...string) {
	GinkgoHelper()
	for _, ns := range names {
		mustRun("kubectl", "create", "ns", ns)
	}
}

// createServer creates a server whose image can't be pulled, so no game image is downloaded.
func createServer(ns string) {
	GinkgoHelper()
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(`apiVersion: %s/v1alpha1
kind: SatisfactoryServer
metadata:
  name: srv
  namespace: %s
spec:
  image: example.invalid/satisfactory-server:none
  serviceType: ClusterIP
  storage:
    size: 1Gi
`, crdGroup, ns))
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
}

func canI(user, verb, resource, ns string) bool {
	GinkgoHelper()
	// kubectl auth can-i exits non-zero for "no".
	out, _ := utils.Run(exec.Command("kubectl", "auth", "can-i", verb, resource, "-n", ns, "--as", user))
	return strings.TrimSpace(out) == "yes"
}

// cleanupHelmInstall deletes the servers while the controller still runs (it removes their finalizers),
// then the release, the kept CRDs and the test namespaces.
func cleanupHelmInstall(namespaces ...string) {
	for _, ns := range namespaces {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "satisfactoryserver", "--all", "-n", ns, "--timeout=2m"))
	}
	_, _ = utils.Run(exec.Command("helm", "uninstall", helmRelease, "-n", namespace, "--wait"))
	for _, name := range crdNames {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "crd", name+"."+crdGroup, "--ignore-not-found"))
	}
	_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", namespace, "--ignore-not-found"))
	for _, ns := range namespaces {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", ns, "--ignore-not-found"))
	}
}
