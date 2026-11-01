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

var _ = Describe("Backups", Ordered, func() {
	SetDefaultEventuallyTimeout(3 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	BeforeAll(func() {
		createNamespaces("games")
		helmInstall()
	})

	AfterAll(func() {
		_, _ = utils.Run(exec.Command("kubectl", "delete", "savegame", "--all", "-n", "games"))
		cleanupHelmInstall("games")
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			for _, args := range [][]string{
				{"logs", "-n", namespace, "-l", "control-plane=controller-manager", "--tail=100"},
				{"get", "satisfactoryserver,savegame,jobs,pods", "-n", "games", "-o", "yaml"},
				{"logs", "-n", "games", "srv-0", "--tail=50"},
			} {
				out, _ := utils.Run(exec.Command("kubectl", args...))
				_, _ = fmt.Fprintln(GinkgoWriter, out)
			}
		}
	})

	It("should back up to the server through a Job when no destinations are set", func() {
		apply(fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: srv-admin
  namespace: games
stringData:
  password: e2e
---
apiVersion: %[1]s/v1alpha1
kind: SatisfactoryServer
metadata:
  name: srv
  namespace: games
spec:
  image: %[2]s
  serviceType: ClusterIP
  adminPasswordSecretRef:
    name: srv-admin
  storage:
    size: 1Gi
  resources:
    requests:
      memory: 32Mi
  # The fake image has no healthcheck.sh.
  readinessProbeOverride:
    tcpSocket:
      port: 7777
  livenessProbeOverride:
    tcpSocket:
      port: 7777
`, crdGroup, fakeGameImage))

		By("waiting for the operator to reach and authenticate against the fake server")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "satisfactoryserver", "srv", "-n", "games", "-o",
				`jsonpath={.status.conditions[?(@.type=="ServerReachable")].status} `+
					`{.status.conditions[?(@.type=="Authenticated")].status}`))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("True True"))
		}).Should(Succeed())

		apply(fmt.Sprintf(`apiVersion: %s/v1alpha1
kind: SaveGame
metadata:
  name: oneshot
  namespace: games
spec:
  serverRef: srv
  saveName: e2e
`, crdGroup))

		By("waiting for the backup Job to complete and the SaveGame to be Ready")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "savegame", "oneshot", "-n", "games", "-o",
				`jsonpath={.status.phase} {.status.conditions[?(@.type=="Ready")].status} `+
					`{.status.destinations[*].name} {.status.retainedSaveCount}`))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("Completed True server 1"))
		}).Should(Succeed())

		jobs := mustRun("kubectl", "get", "jobs", "-n", "games", "-o",
			`jsonpath={range .items[*]}{.metadata.ownerReferences[0].name} {.status.succeeded}{"\n"}{end}`)
		Expect(strings.TrimSpace(jobs)).To(Equal("oneshot 1"))

		By("checking that the save was made on the game server")
		Expect(mustRun("kubectl", "logs", "srv-0", "-n", "games")).To(MatchRegexp(`SaveGame e2e_\d{8}-\d{6}`))
	})

	It("should write the save to a pvc destination", func() {
		// Kind's default storage class (local-path, RWO); the server from the previous spec is reused.
		apply(fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: backups
  namespace: games
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Mi
---
apiVersion: %s/v1alpha1
kind: SaveGame
metadata:
  name: topvc
  namespace: games
spec:
  serverRef: srv
  saveName: e2epvc
  destinations:
  - name: nas
    pvc:
      claimName: backups
      path: factory
`, crdGroup))

		By("waiting for the backup Job to complete and the SaveGame to be Ready")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "savegame", "topvc", "-n", "games", "-o",
				`jsonpath={.status.phase} {.status.conditions[?(@.type=="Ready")].status} `+
					`{.status.destinations[*].name} {.status.destinations[*].retainedCount}`))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("Completed True nas 1"))
		}).Should(Succeed())

		By("listing the claim from another pod")
		apply(`apiVersion: v1
kind: Pod
metadata:
  name: ls-backups
  namespace: games
spec:
  restartPolicy: Never
  containers:
  - name: ls
    image: curlimages/curl:latest
    command: [ls, /data/factory]
    volumeMounts:
    - name: backups
      mountPath: /data
  volumes:
  - name: backups
    persistentVolumeClaim:
      claimName: backups
`)
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "pod", "ls-backups", "-n", "games",
				"-o", "jsonpath={.status.phase}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("Succeeded"))
		}).Should(Succeed())
		Expect(strings.TrimSpace(mustRun("kubectl", "logs", "ls-backups", "-n", "games"))).
			To(MatchRegexp(`^e2epvc_\d{8}-\d{6}\.sav$`))
	})

	It("should upload the save to an s3 destination", func() {
		apply(`apiVersion: v1
kind: Secret
metadata:
  name: s3-creds
  namespace: games
stringData:
  AWS_ACCESS_KEY_ID: e2ekey
  AWS_SECRET_ACCESS_KEY: e2esecret123
---
apiVersion: v1
kind: Pod
metadata:
  name: rustfs
  namespace: games
  labels:
    app: rustfs
spec:
  containers:
  - name: rustfs
    image: rustfs/rustfs:latest
    env:
    - name: RUSTFS_ACCESS_KEY
      value: e2ekey
    - name: RUSTFS_SECRET_KEY
      value: e2esecret123
    ports:
    - containerPort: 9000
    readinessProbe:
      tcpSocket:
        port: 9000
---
apiVersion: v1
kind: Service
metadata:
  name: rustfs
  namespace: games
spec:
  selector:
    app: rustfs
  ports:
  - port: 9000
`)
		mustRun("kubectl", "wait", "pod/rustfs", "-n", "games", "--for=condition=Ready", "--timeout=3m")

		awsCLI := func(name string, args ...string) string {
			GinkgoHelper()
			quoted := `"` + strings.Join(args, `", "`) + `"`
			apply(fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: games
spec:
  restartPolicy: Never
  containers:
  - name: aws
    image: amazon/aws-cli:latest
    args: ["--endpoint-url", "http://rustfs.games:9000", %s]
    envFrom:
    - secretRef:
        name: s3-creds
    env:
    - name: AWS_DEFAULT_REGION
      value: us-east-1
`, name, quoted))
			Eventually(func(g Gomega) {
				out, err := utils.Run(exec.Command("kubectl", "get", "pod", name, "-n", "games",
					"-o", "jsonpath={.status.phase}"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(out).To(Equal("Succeeded"))
			}).Should(Succeed())
			return mustRun("kubectl", "logs", name, "-n", "games")
		}

		By("creating the bucket")
		awsCLI("s3-mb", "s3", "mb", "s3://backups")

		apply(fmt.Sprintf(`apiVersion: %s/v1alpha1
kind: SaveGame
metadata:
  name: tos3
  namespace: games
spec:
  serverRef: srv
  saveName: e2es3
  destinations:
  - name: offsite
    s3:
      bucket: backups
      prefix: factory
      endpoint: http://rustfs.games:9000
      forcePathStyle: true
      credentialsSecretRef:
        name: s3-creds
`, crdGroup))

		By("waiting for the backup Job to complete and the SaveGame to be Ready")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "savegame", "tos3", "-n", "games", "-o",
				`jsonpath={.status.phase} {.status.conditions[?(@.type=="Ready")].status} `+
					`{.status.destinations[*].name} {.status.destinations[*].retainedCount}`))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("Completed True offsite 1"))
		}).Should(Succeed())

		By("listing the bucket")
		Expect(awsCLI("s3-ls", "s3", "ls", "s3://backups/factory/")).
			To(MatchRegexp(`\se2es3_\d{8}-\d{6}\.sav\n?$`))
	})
})

// apply pipes manifests into kubectl apply.
func apply(manifest string) {
	GinkgoHelper()
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
}
