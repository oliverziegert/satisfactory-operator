## 1. Project Scaffolding

- [x] 1.1 Run `operator-sdk init --domain oliverziegert.github.com --repo github.com/oliverziegert/satisfactory-operator` to scaffold the project
- [x] 1.2 Verify generated project structure: go.mod, Makefile, Dockerfile, main.go, config/ directory with Kustomize manifests
- [x] 1.3 Run `go build ./...` to confirm the scaffolded project compiles

## 2. SatisfactoryServer API and Controller

- [x] 2.1 Run `operator-sdk create api --group satisfactory --version v1alpha1 --kind SatisfactoryServer --resource --controller`
- [x] 2.2 Define SatisfactoryServer spec fields: server config (name, admin password, max players, auto-pause, auto-save interval), resource requirements, persistent storage (size, storage class), networking (game port, query port, API port), image version
- [x] 2.3 Define SatisfactoryServer status fields: phase (Pending/Running/Updating/Error), conditions (Available/Progressing/Degraded), server version, connected players, last save timestamp
- [x] 2.4 Add kubebuilder RBAC markers for Pods, Services, PVCs, ConfigMaps, and SatisfactoryServer resources
- [x] 2.5 Implement basic reconciliation stub that logs the request and returns

## 3. SaveGame API and Controller

- [x] 3.1 Run `operator-sdk create api --group satisfactory --version v1alpha1 --kind SaveGame --resource --controller`
- [x] 3.2 Define SaveGame spec fields: server reference, save name, retention policy (max count, max age), schedule (cron expression)
- [x] 3.3 Define SaveGame status fields: phase (Pending/Active/Completed/Failed), last backup timestamp, backup size, conditions
- [x] 3.4 Add kubebuilder RBAC markers for SaveGame resources and read access to SatisfactoryServer
- [x] 3.5 Implement basic reconciliation stub that logs the request and returns

## 4. ClusterPolicy API and Controller

- [x] 4.1 Run `operator-sdk create api --group satisfactory --version v1alpha1 --kind ClusterPolicy --resource --controller`
- [x] 4.2 Mark ClusterPolicy as cluster-scoped using `// +kubebuilder:resource:scope=Cluster`
- [x] 4.3 Define ClusterPolicy spec fields: default resource limits, default storage class, default server config, allowed image versions, max servers per namespace
- [x] 4.4 Define ClusterPolicy status fields: conditions (Ready/Degraded), servers governed count, last applied timestamp
- [x] 4.5 Add kubebuilder RBAC markers for ClusterPolicy resources and cluster-wide SatisfactoryServer read access
- [x] 4.6 Implement basic reconciliation stub that logs the request and returns

## 5. Generate and Verify

- [x] 5.1 Run `make generate` to generate DeepCopy methods and other code
- [x] 5.2 Run `make manifests` to generate CRD YAML manifests and RBAC roles
- [x] 5.3 Verify CRD manifests exist in config/crd/bases/ for all three resources
- [x] 5.4 Run `make build` to confirm everything compiles
- [x] 5.5 Run `make test` to confirm generated tests pass