# helm-chart Specification

## Purpose
Lets users install, configure and upgrade the operator with Helm, including GitOps tools, in either cluster-wide or namespace-scoped mode.

## Requirements
### Requirement: Operator installation via Helm
The chart SHALL install the CRDs, RBAC, ServiceAccount and controller Deployment so that the operator reconciles resources after `helm install` with default values. The chart SHALL NOT create `SatisfactoryServer`, `SaveGame` or Secret resources for game servers.

#### Scenario: Default install
- **WHEN** a user runs `helm install sat oci://ghcr.io/oliverziegert/charts/satisfactory-operator --version X.Y.Z -n satisfactory-operator-system --create-namespace`
- **THEN** the controller Deployment runs `ghcr.io/oliverziegert/satisfactory-operator:vX.Y.Z` and reconciles a `SatisfactoryServer` in any namespace

### Requirement: Configurable values
The chart SHALL let users set, under `manager`, the image repository and tag, image pull secrets, resources, node selector, tolerations, affinity and security contexts, plus top-level toggles for metrics, ServiceMonitor (`prometheus`) and NetworkPolicy. The image tag SHALL default to the chart's `appVersion`.

#### Scenario: Custom resources and scheduling
- **WHEN** a user sets `manager.resources.limits.memory=256Mi` and `manager.nodeSelector.kubernetes\.io/arch=arm64`
- **THEN** the rendered Deployment carries that memory limit and node selector

### Requirement: CRD lifecycle
With `crd.enabled` (default), CRDs SHALL be created on install and updated on `helm upgrade`. With `crd.keep` (default), `helm uninstall` SHALL NOT delete the CRDs or any custom resources.

#### Scenario: Schema change on upgrade
- **WHEN** a new chart version adds a field to a CRD and the user runs `helm upgrade`
- **THEN** the CRD in the cluster contains the new field

#### Scenario: Uninstall keeps servers
- **WHEN** a user runs `helm uninstall` while `SatisfactoryServer` resources exist
- **THEN** the CRDs and the `SatisfactoryServer` resources still exist

### Requirement: Optional ClusterPolicy
With `clusterPolicy.enabled: true`, the chart SHALL create one `ClusterPolicy` whose spec is `clusterPolicy.spec`, passed through unchanged. It SHALL be created after the CRDs exist and updated on every upgrade. It is disabled by default.

#### Scenario: First install with policy
- **WHEN** a user installs with `clusterPolicy.enabled=true` and `clusterPolicy.spec.maxServersPerNamespace=2`
- **THEN** the install succeeds in one `helm install` and a `ClusterPolicy` with `maxServersPerNamespace: 2` exists

### Requirement: Namespace-scoped mode
With `watchNamespaces` empty, the chart SHALL render cluster-wide RBAC. With a list, it SHALL grant the manager's rules per listed namespace through RoleBindings and configure the operator to watch exactly those namespaces. Cluster-level permissions SHALL be limited to `clusterpolicies` and their subresources, plus the TokenReview and SubjectAccessReview creation that secure metrics need. The chart SHALL NOT create the listed namespaces.

#### Scenario: Scoped install
- **WHEN** a user installs with `watchNamespaces={games}`
- **THEN** the operator's ServiceAccount can manage Secrets and StatefulSets in `games`, cannot read Secrets in any other namespace, and a `SatisfactoryServer` in `games` gets a StatefulSet

#### Scenario: Server outside the watched namespaces
- **WHEN** a user installs with `watchNamespaces={games}` and creates a `SatisfactoryServer` in `other`
- **THEN** no StatefulSet is created for it

### Requirement: Migration from install.yaml
The README SHALL document a migration from an `install.yaml` installation to the chart that keeps the CRDs and all custom resources, and SHALL warn that `kubectl delete -f install.yaml` deletes all servers.

#### Scenario: Documented migration
- **WHEN** a user with a running `SatisfactoryServer` installed via `install.yaml` follows the documented migration steps
- **THEN** after `helm install` the server's StatefulSet and PVC still exist and the server is reconciled by the new controller
