## ADDED Requirements

### Requirement: ClusterPolicy CRD definition
The system SHALL define a `ClusterPolicy` custom resource in API group `satisfactory.oliverziegert.github.com/v1alpha1` that provides cluster-wide default configuration for Satisfactory servers.

#### Scenario: CRD API generation
- **WHEN** `operator-sdk create api --group satisfactory --version v1alpha1 --kind ClusterPolicy --resource --controller` is run
- **THEN** the project SHALL contain Go type definitions at `api/v1alpha1/clusterpolicy_types.go` and a controller at `internal/controller/clusterpolicy_controller.go`

#### Scenario: ClusterPolicy spec fields
- **WHEN** a `ClusterPolicy` custom resource is defined
- **THEN** the spec SHALL include fields for: default resource limits (CPU, memory), default storage class, default server configuration values, allowed image versions, and maximum servers per namespace

#### Scenario: ClusterPolicy status fields
- **WHEN** the operator reconciles a `ClusterPolicy` resource
- **THEN** the status SHALL report: conditions (Ready, Degraded), number of servers governed, and last applied timestamp

### Requirement: ClusterPolicy is cluster-scoped
The `ClusterPolicy` resource SHALL be cluster-scoped (not namespaced) to apply defaults across all namespaces.

#### Scenario: Cluster-wide visibility
- **WHEN** a `ClusterPolicy` resource is created
- **THEN** it SHALL be accessible without a namespace qualifier and apply to `SatisfactoryServer` resources in all namespaces

### Requirement: ClusterPolicy controller reconciliation stub
The ClusterPolicy controller SHALL implement the `Reconcile` method with a basic reconciliation loop that logs the reconciliation request and returns.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the ClusterPolicy controller SHALL be registered with the manager and begin watching `ClusterPolicy` resources

### Requirement: ClusterPolicy RBAC configuration
The operator SHALL define cluster-level RBAC rules for the ClusterPolicy controller granting access to manage ClusterPolicy custom resources and read SatisfactoryServer resources across all namespaces.

#### Scenario: RBAC markers
- **WHEN** the controller source code is processed by controller-gen
- **THEN** ClusterRole RBAC rules SHALL be generated from `// +kubebuilder:rbac` markers on the ClusterPolicy controller
