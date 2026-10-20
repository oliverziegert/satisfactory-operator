## ADDED Requirements

### Requirement: SaveGame CRD definition
The system SHALL define a `SaveGame` custom resource in API group `satisfactory.oliverziegert.github.com/v1alpha1` that represents a save game backup for a Satisfactory server.

#### Scenario: CRD API generation
- **WHEN** `operator-sdk create api --group satisfactory --version v1alpha1 --kind SaveGame --resource --controller` is run
- **THEN** the project SHALL contain Go type definitions at `api/v1alpha1/savegame_types.go` and a controller at `internal/controller/savegame_controller.go`

#### Scenario: SaveGame spec fields
- **WHEN** a `SaveGame` custom resource is defined
- **THEN** the spec SHALL include fields for: server reference (name of the SatisfactoryServer resource), save name, retention policy (max count, max age), and schedule (cron expression for automated backups)

#### Scenario: SaveGame status fields
- **WHEN** the operator reconciles a `SaveGame` resource
- **THEN** the status SHALL report: phase (Pending, Active, Completed, Failed), last backup timestamp, backup size, and conditions

### Requirement: SaveGame is namespaced
The `SaveGame` resource SHALL be namespace-scoped and SHALL reference a `SatisfactoryServer` in the same namespace.

#### Scenario: Same-namespace reference
- **WHEN** a `SaveGame` references a `SatisfactoryServer` by name
- **THEN** the operator SHALL resolve the reference within the same namespace as the `SaveGame` resource

### Requirement: SaveGame controller reconciliation stub
The SaveGame controller SHALL implement the `Reconcile` method with a basic reconciliation loop that logs the reconciliation request and returns.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the SaveGame controller SHALL be registered with the manager and begin watching `SaveGame` resources

### Requirement: SaveGame RBAC configuration
The operator SHALL define RBAC rules for the SaveGame controller granting access to manage SaveGame custom resources and read SatisfactoryServer resources.

#### Scenario: RBAC markers
- **WHEN** the controller source code is processed by controller-gen
- **THEN** RBAC rules SHALL be generated from `// +kubebuilder:rbac` markers on the SaveGame controller
