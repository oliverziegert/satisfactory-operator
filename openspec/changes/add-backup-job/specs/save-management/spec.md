# Spec Delta

## MODIFIED Requirements

### Requirement: SaveGame CRD definition
The system SHALL define a `SaveGame` custom resource in API group `satisfactory.oliverziegert.github.com/v1alpha1` that represents a save game backup for a Satisfactory server.

#### Scenario: CRD API generation
- **WHEN** `operator-sdk create api --group satisfactory --version v1alpha1 --kind SaveGame --resource --controller` is run
- **THEN** the project SHALL contain Go type definitions at `api/v1alpha1/savegame_types.go` and a controller at `internal/controller/savegame_controller.go`

#### Scenario: SaveGame spec fields
- **WHEN** a `SaveGame` custom resource is defined
- **THEN** the spec SHALL include fields for: server reference (name of the SatisfactoryServer resource), save name, retention policy (max count, max age), schedule (cron expression for automated backups), and destinations (a list of named backup destinations, each with an optional retention policy)

#### Scenario: SaveGame status fields
- **WHEN** the operator reconciles a `SaveGame` resource
- **THEN** the status SHALL report: phase (Pending, Active, Completed, Failed), last backup timestamp, backup size, conditions, and per-destination results (`status.destinations[]` with name, last success time, last size, retained count and last error)

### Requirement: SaveGame controller reconciliation stub
The SaveGame controller SHALL implement the `Reconcile` method with a full reconciliation loop. The loop resolves the referenced SatisfactoryServer, makes sure a valid auth token is cached, creates a backup Job when a backup is due, reads the result of finished Jobs into status, and updates status conditions and phase.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the SaveGame controller SHALL be registered with the manager and begin watching `SaveGame` resources, referenced `SatisfactoryServer` resources, and Jobs owned by SaveGames

#### Scenario: Full reconciliation with scheduled backup
- **WHEN** a `SaveGame` resource is reconciled, a backup is due per the cron schedule, and no owned Job is running
- **THEN** the controller SHALL authenticate, create a backup Job, and requeue when the Job changes

#### Scenario: Backup Job finished
- **WHEN** an owned backup Job has finished and its result is not yet in status
- **THEN** the controller SHALL update `lastBackupTime`, `backupSize` and `status.destinations` from the result, and requeue at the next scheduled time

#### Scenario: Reconciliation without backup due
- **WHEN** a `SaveGame` resource is reconciled and no backup is due
- **THEN** the controller SHALL verify server reachability and requeue at the next scheduled time, without calling the game API

### Requirement: SaveGame RBAC configuration
The operator SHALL define RBAC rules for the SaveGame controller granting access to manage SaveGame custom resources, read SatisfactoryServer resources, manage backup Jobs, and read the pods of those Jobs.

#### Scenario: RBAC markers
- **WHEN** the controller source code is processed by controller-gen
- **THEN** RBAC rules SHALL be generated from `// +kubebuilder:rbac` markers on the SaveGame controller, including `batch` `jobs` with create, get, list, watch and delete

#### Scenario: Namespace mode
- **WHEN** the operator is installed through Helm with `watchNamespaces` set
- **THEN** the Job permissions SHALL apply in each watched namespace through the existing per-namespace binding of the manager role

### Requirement: SaveGame status fields for operational state
The `SaveGameStatus` SHALL include additional fields: `nextBackupTime` (metav1.Time, optional) indicating when the next scheduled backup will occur, `retainedSaveCount` (int32, optional) indicating how many saves the `server` destination holds for this session, and `destinations` (list, optional) with per-destination results.

#### Scenario: Status populated after reconciliation
- **WHEN** a SaveGame with a schedule finishes a backup Job
- **THEN** `status.nextBackupTime` SHALL be set to the next cron trigger time, `status.retainedSaveCount` SHALL reflect the `server` destination's retained count from the Job result (0 without a `server` destination), and `status.destinations` SHALL have one entry per configured destination
