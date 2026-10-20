# server-lifecycle Specification

## Purpose
Scaffold the operator and define the SatisfactoryServer resource, its controller and RBAC.

## Requirements
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
- **THEN** the spec SHALL include fields for: server configuration (server name, admin password, max players, auto-pause, auto-save interval), resource requirements (CPU, memory requests/limits), persistent storage (size, storage class), networking (game port, query port, API port), image version, `adminPasswordSecretRef`, and the new fields `serverName` (string, optional) and `autoClaimDisabled` (bool, optional, default false)

#### Scenario: SatisfactoryServer status fields
- **WHEN** the operator reconciles a `SatisfactoryServer` resource
- **THEN** the status SHALL report: phase (Pending, Running, Updating, Error), conditions (Available, Progressing, Degraded), server version, connected players count, and last save timestamp

### Requirement: SatisfactoryServer is namespaced
The `SatisfactoryServer` resource SHALL be namespace-scoped to allow multi-tenant operation and per-namespace RBAC.

#### Scenario: Multi-namespace deployment
- **WHEN** `SatisfactoryServer` resources are created in different namespaces
- **THEN** each resource SHALL be independently reconciled within its own namespace

### Requirement: Controller reconciliation stub
The SatisfactoryServer controller SHALL implement the `Reconcile` method with a full reconciliation loop that creates and manages a StatefulSet, headless Service, and external Service, adds a finalizer, handles deletion, authenticates with the game server API when credentials are configured, claims unclaimed servers when auto-claiming is enabled, updates status conditions by probing the game server API, and updates the resource's Phase status.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the SatisfactoryServer controller SHALL be registered with the manager and begin watching `SatisfactoryServer` resources

#### Scenario: Full reconciliation on create
- **WHEN** a `SatisfactoryServer` resource is created
- **THEN** the controller SHALL add a finalizer, create a StatefulSet, a headless Service, and an external Service in the same namespace, and set initial conditions (`Ready=False`, `Progressing=True`, `ServerReachable=False`, `Authenticated=Unknown`)

#### Scenario: Claiming step in reconciliation
- **WHEN** the probe determines `ServerClaimed=False` and `ServerReachable=True` and auto-claiming is enabled
- **THEN** the controller SHALL attempt to claim the server before completing the reconciliation loop

#### Scenario: Phase set to Pending while server starts
- **WHEN** a `SatisfactoryServer` resource is reconciled and the StatefulSet has zero ready replicas
- **THEN** the controller SHALL set `status.phase` to `Pending` and `Progressing=True`

#### Scenario: Phase set to Running when server is ready
- **WHEN** the StatefulSet managed by a `SatisfactoryServer` has at least one ready replica AND the game server API is reachable
- **THEN** the controller SHALL set `status.phase` to `Running` and update all game-specific conditions

#### Scenario: Game server API probing during reconciliation
- **WHEN** the StatefulSet has ready replicas
- **THEN** the controller SHALL authenticate if configured, probe the server, attempt claiming if unclaimed and enabled, then update all conditions accordingly

#### Scenario: Status fields populated from game API
- **WHEN** `QueryServerState` returns successfully
- **THEN** the controller SHALL populate `status.connectedPlayers` and `status.serverVersion` from the response

### Requirement: RBAC configuration
The operator SHALL define RBAC rules granting least-privilege access to manage Pods, Services, PersistentVolumeClaims, ConfigMaps, and SatisfactoryServer custom resources.

#### Scenario: RBAC markers
- **WHEN** the controller source code is processed by controller-gen
- **THEN** RBAC ClusterRole manifests SHALL be generated from `// +kubebuilder:rbac` markers on the controller

### Requirement: RBAC for StatefulSets and Services
The SatisfactoryServer controller SHALL have RBAC permissions to create, get, list, watch, update, patch, and delete `StatefulSets` and `Services` in addition to the existing Pod, PVC, ConfigMap, and SatisfactoryServer permissions.

#### Scenario: RBAC markers for new resources
- **WHEN** the controller source code is processed by controller-gen
- **THEN** the generated ClusterRole SHALL include rules for `apps/statefulsets` and `core/services`
