# cluster-policy Specification

## Purpose
Define the cluster-scoped ClusterPolicy resource, its controller and RBAC, used to set defaults and constraints for SatisfactoryServer resources.

## Requirements
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
- **THEN** the status SHALL report: conditions (Ready, Enforced), number of servers governed (`serversGoverned`), and last applied timestamp (`lastAppliedTime` set to current time on each reconciliation)

#### Scenario: Status populated after reconciliation
- **WHEN** there are 5 SatisfactoryServer resources across the cluster
- **THEN** `status.serversGoverned` SHALL be `5` and `status.lastAppliedTime` SHALL be the current reconciliation time

### Requirement: ClusterPolicy is cluster-scoped
The `ClusterPolicy` resource SHALL be cluster-scoped (not namespaced) to apply defaults across all namespaces.

#### Scenario: Cluster-wide visibility
- **WHEN** a `ClusterPolicy` resource is created
- **THEN** it SHALL be accessible without a namespace qualifier and apply to `SatisfactoryServer` resources in all namespaces

### Requirement: ClusterPolicy controller reconciliation stub
The ClusterPolicy controller SHALL implement the `Reconcile` method with a full reconciliation loop that lists all SatisfactoryServer resources, applies defaults, validates constraints, updates conditions, and populates status fields.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the ClusterPolicy controller SHALL be registered with the manager and begin watching `ClusterPolicy` and `SatisfactoryServer` resources

#### Scenario: Full reconciliation
- **WHEN** a ClusterPolicy resource is reconciled
- **THEN** the controller SHALL list all SatisfactoryServer resources across all namespaces, apply defaults to servers with zero-valued fields, validate image and namespace constraints, set `Enforced` and `Ready` conditions, and populate `serversGoverned` and `lastAppliedTime`

### Requirement: ClusterPolicy RBAC configuration
The operator SHALL define cluster-level RBAC rules for the ClusterPolicy controller granting access to manage ClusterPolicy custom resources, read and patch SatisfactoryServer resources across all namespaces, and create Events.

#### Scenario: RBAC markers
- **WHEN** the controller source code is processed by controller-gen
- **THEN** ClusterRole RBAC rules SHALL include `patch` verb for SatisfactoryServer resources and `create` verb for Events

### Requirement: ClusterPolicy Enforced condition
The `Enforced` condition SHALL reflect whether all governed servers comply with the policy constraints.

#### Scenario: All servers compliant
- **WHEN** all SatisfactoryServer resources use allowed images and no namespace exceeds the server limit
- **THEN** the `Enforced` condition SHALL be `True` with reason `AllServersCompliant`

#### Scenario: Non-compliant servers exist
- **WHEN** one or more SatisfactoryServer resources violate the policy (disallowed image or namespace limit exceeded)
- **THEN** the `Enforced` condition SHALL be `False` with reason `PolicyViolation` and a message listing the non-compliant servers
