# save-management Specification

## Purpose
Manage Satisfactory save games as Kubernetes resources: the SaveGame CRD, its controller and its RBAC.

## Requirements
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
The SaveGame controller SHALL implement the `Reconcile` method with a full reconciliation loop that resolves the referenced SatisfactoryServer, authenticates with the game API, triggers saves on schedule, enforces retention policies, and updates status conditions and phase.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the SaveGame controller SHALL be registered with the manager and begin watching `SaveGame` resources and referenced `SatisfactoryServer` resources

#### Scenario: Full reconciliation with scheduled backup
- **WHEN** a `SaveGame` resource is reconciled and a backup is due per the cron schedule
- **THEN** the controller SHALL authenticate, call `SaveGame`, enforce retention, update `lastBackupTime`, and requeue at the next scheduled time

#### Scenario: Reconciliation without backup due
- **WHEN** a `SaveGame` resource is reconciled and no backup is due
- **THEN** the controller SHALL verify server reachability, update status from `EnumerateSessions`, and requeue at the next scheduled time

### Requirement: SaveGame RBAC configuration
The operator SHALL define RBAC rules for the SaveGame controller granting access to manage SaveGame custom resources and read SatisfactoryServer resources.

#### Scenario: RBAC markers
- **WHEN** the controller source code is processed by controller-gen
- **THEN** RBAC rules SHALL be generated from `// +kubebuilder:rbac` markers on the SaveGame controller

### Requirement: SaveGame status fields for operational state
The `SaveGameStatus` SHALL include additional fields: `nextBackupTime` (metav1.Time, optional) indicating when the next scheduled backup will occur, and `retainedSaveCount` (int32, optional) indicating how many saves currently exist for this session.

#### Scenario: Status populated after reconciliation
- **WHEN** a SaveGame with a schedule is reconciled successfully
- **THEN** `status.nextBackupTime` SHALL be set to the next cron trigger time and `status.retainedSaveCount` SHALL reflect the number of saves returned by `EnumerateSessions` for the session

### Requirement: SaveGame watches SatisfactoryServer
The SaveGame controller SHALL watch SatisfactoryServer resources and re-reconcile SaveGames that reference a changed server.

#### Scenario: Server becomes reachable
- **WHEN** a SatisfactoryServer transitions to `ServerReachable=True`
- **THEN** all SaveGame resources with `serverRef` pointing to that server SHALL be re-reconciled

### Requirement: SaveGame Phase reflects operational state
The SaveGame Phase SHALL be derived from the controller's operational state, not just whether reconciliation succeeded.

#### Scenario: Phase Active
- **WHEN** a schedule is configured and the last backup succeeded
- **THEN** `Phase` SHALL be `Active`

#### Scenario: Phase Pending
- **WHEN** the referenced server is not reachable or not authenticated
- **THEN** `Phase` SHALL be `Pending`

#### Scenario: Phase Failed
- **WHEN** the last save operation or retention enforcement failed
- **THEN** `Phase` SHALL be `Failed`

#### Scenario: Phase Completed
- **WHEN** no schedule is configured and the last triggered save succeeded
- **THEN** `Phase` SHALL be `Completed`
