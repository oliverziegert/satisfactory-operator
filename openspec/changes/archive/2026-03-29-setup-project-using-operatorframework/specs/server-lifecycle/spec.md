## ADDED Requirements

### Requirement: Operator project scaffolding
The system SHALL be scaffolded using `operator-sdk init` with Go as the project language, producing a standard project layout with Makefile, Dockerfile, go.mod, and Kustomize manifests.

#### Scenario: Project initialization
- **WHEN** the operator project is initialized with `operator-sdk init --domain oliverziegert.github.com --repo github.com/oliverziegert/satisfactory-operator`
- **THEN** the project SHALL contain a valid Go module, a Makefile with standard targets (build, test, docker-build, install, deploy), a multi-stage Dockerfile, and Kustomize base manifests

### Requirement: SatisfactoryServer CRD definition
The system SHALL define a `SatisfactoryServer` custom resource in API group `satisfactory.oliverziegert.github.com/v1alpha1` that represents a single Satisfactory Dedicated Server instance.

#### Scenario: CRD API generation
- **WHEN** `operator-sdk create api --group satisfactory --version v1alpha1 --kind SatisfactoryServer --resource --controller` is run
- **THEN** the project SHALL contain Go type definitions at `api/v1alpha1/satisfactoryserver_types.go` and a controller at `internal/controller/satisfactoryserver_controller.go`

#### Scenario: SatisfactoryServer spec fields
- **WHEN** a `SatisfactoryServer` custom resource is defined
- **THEN** the spec SHALL include fields for: server configuration (server name, admin password, max players, auto-pause, auto-save interval), resource requirements (CPU, memory requests/limits), persistent storage (size, storage class), networking (game port, query port, API port), and image version

#### Scenario: SatisfactoryServer status fields
- **WHEN** the operator reconciles a `SatisfactoryServer` resource
- **THEN** the status SHALL report: phase (Pending, Running, Updating, Error), conditions (Available, Progressing, Degraded), server version, connected players count, and last save timestamp

### Requirement: SatisfactoryServer is namespaced
The `SatisfactoryServer` resource SHALL be namespace-scoped to allow multi-tenant operation and per-namespace RBAC.

#### Scenario: Multi-namespace deployment
- **WHEN** `SatisfactoryServer` resources are created in different namespaces
- **THEN** each resource SHALL be independently reconciled within its own namespace

### Requirement: Controller reconciliation stub
The SatisfactoryServer controller SHALL implement the `Reconcile` method with a basic reconciliation loop that logs the reconciliation request and returns.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the SatisfactoryServer controller SHALL be registered with the manager and begin watching `SatisfactoryServer` resources

### Requirement: RBAC configuration
The operator SHALL define RBAC rules granting least-privilege access to manage Pods, Services, PersistentVolumeClaims, ConfigMaps, and SatisfactoryServer custom resources.

#### Scenario: RBAC markers
- **WHEN** the controller source code is processed by controller-gen
- **THEN** RBAC ClusterRole manifests SHALL be generated from `// +kubebuilder:rbac` markers on the controller