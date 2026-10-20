# status-conditions Specification

## Purpose
Status conditions, phase and status fields that report the observed state of SatisfactoryServer, SaveGame and ClusterPolicy resources.

## Requirements
### Requirement: Condition type constants for SatisfactoryServer
The system SHALL define the following condition type constants for SatisfactoryServer: `Ready`, `Progressing`, `Degraded`, `ServerReachable`, `ServerClaimed`, `WorldLoaded`, `Authenticated`.

#### Scenario: Constants are defined in API package
- **WHEN** the `api/v1alpha1` package is compiled
- **THEN** string constants SHALL exist for each condition type prefixed with `ConditionType` (e.g., `ConditionTypeReady = "Ready"`), including `SatisfactoryServerConditionTypeAuthenticated = "Authenticated"`

### Requirement: Condition type constants for SaveGame
The system SHALL define the following condition type constants for SaveGame: `Ready`, `BackupScheduled`.

#### Scenario: SaveGame condition constants exist
- **WHEN** the `api/v1alpha1` package is compiled
- **THEN** string constants `ConditionTypeReady` and `ConditionTypeBackupScheduled` SHALL exist for SaveGame conditions

### Requirement: Condition type constants for ClusterPolicy
The system SHALL define the following condition type constants for ClusterPolicy: `Ready`, `Enforced`.

#### Scenario: ClusterPolicy condition constants exist
- **WHEN** the `api/v1alpha1` package is compiled
- **THEN** string constants `ConditionTypeReady` and `ConditionTypeEnforced` SHALL exist for ClusterPolicy conditions

### Requirement: Condition helper package
The system SHALL provide a shared helper package at `internal/conditions/` that wraps `apimachinery/pkg/api/meta` condition utilities for type-safe condition management across all CRDs.

#### Scenario: Setting a condition on SatisfactoryServer
- **WHEN** a controller calls the helper to set condition `Ready` with status `True`, reason `AllChecksPass`, and a message
- **THEN** the SatisfactoryServer's `status.conditions` array SHALL contain an entry with `type=Ready`, `status=True`, `reason=AllChecksPass`, `observedGeneration` matching the resource's generation, and a valid `lastTransitionTime`

#### Scenario: Condition transition updates lastTransitionTime
- **WHEN** a condition status changes from `True` to `False`
- **THEN** `lastTransitionTime` SHALL be updated to the current time

#### Scenario: Condition status unchanged preserves lastTransitionTime
- **WHEN** a condition is set with the same status as its current value
- **THEN** `lastTransitionTime` SHALL NOT be updated

### Requirement: SatisfactoryServer Ready condition aggregation
The `Ready` condition on SatisfactoryServer SHALL be `True` only when the StatefulSet has ready replicas AND `ServerReachable` is `True` AND `WorldLoaded` is `True`.

#### Scenario: All sub-conditions true
- **WHEN** the StatefulSet has ready replicas AND the game server API responds successfully AND an active session is loaded
- **THEN** the `Ready` condition SHALL be `True` with reason `AllChecksPass`

#### Scenario: Server unreachable
- **WHEN** the StatefulSet has ready replicas but the game server API is unreachable
- **THEN** the `Ready` condition SHALL be `False` with reason `ServerUnreachable`

#### Scenario: No active session
- **WHEN** the server is reachable but no active session is loaded
- **THEN** the `Ready` condition SHALL be `False` with reason `NoActiveSession`

### Requirement: Progressing condition tracks StatefulSet rollout
The `Progressing` condition SHALL be `True` when the StatefulSet's `updatedReplicas` does not equal `readyReplicas`, indicating a rollout is in progress.

#### Scenario: Rollout in progress
- **WHEN** the StatefulSet has `updatedReplicas=1` and `readyReplicas=0`
- **THEN** the `Progressing` condition SHALL be `True` with reason `RolloutInProgress`

#### Scenario: Rollout complete
- **WHEN** the StatefulSet has `updatedReplicas=1` and `readyReplicas=1`
- **THEN** the `Progressing` condition SHALL be `False` with reason `RolloutComplete`

### Requirement: Degraded condition reflects API failures
The `Degraded` condition SHALL be `True` when the StatefulSet has ready replicas but the game server HTTPS API is not responding or returning errors.

#### Scenario: API connection failure
- **WHEN** the StatefulSet has ready replicas AND the game server API connection times out or is refused
- **THEN** the `Degraded` condition SHALL be `True` with reason `APIUnreachable` and a message containing the error

#### Scenario: API healthy
- **WHEN** the game server API responds successfully
- **THEN** the `Degraded` condition SHALL be `False` with reason `APIHealthy`

### Requirement: ServerReachable condition
The `ServerReachable` condition SHALL be `True` when the game server's HTTPS API responds to a health query, regardless of authentication state.

#### Scenario: API responds with success
- **WHEN** `QueryServerState` returns a successful response
- **THEN** `ServerReachable` SHALL be `True` with reason `APIResponding`

#### Scenario: API responds with auth error
- **WHEN** `QueryServerState` returns a 401 or 403 error
- **THEN** `ServerReachable` SHALL be `True` with reason `APIRespondingAuthRequired`

#### Scenario: API unreachable
- **WHEN** `QueryServerState` returns a connection error or timeout
- **THEN** `ServerReachable` SHALL be `False` with reason `APIConnectionFailed`

### Requirement: ServerClaimed condition
The `ServerClaimed` condition SHALL reflect whether the Satisfactory server has been claimed by an administrator. The controller MAY transition this condition to `True` by automatically claiming the server.

#### Scenario: Server is claimed
- **WHEN** `QueryServerState` succeeds and the server state indicates it has been claimed
- **THEN** `ServerClaimed` SHALL be `True` with reason `ServerClaimed`

#### Scenario: Server is unclaimed
- **WHEN** the server API returns an auth error indicating the server is in initial setup state
- **THEN** `ServerClaimed` SHALL be `False` with reason `ServerUnclaimed`

#### Scenario: Server claimed by controller
- **WHEN** the controller successfully calls `ClaimServer` on an unclaimed server
- **THEN** `ServerClaimed` SHALL transition to `True` with reason `AutoClaimed` on the next reconciliation

### Requirement: WorldLoaded condition
The `WorldLoaded` condition SHALL be `True` when the game server has an active session loaded.

#### Scenario: Active session present
- **WHEN** `QueryServerState` returns a non-empty `activeSessionName`
- **THEN** `WorldLoaded` SHALL be `True` with reason `SessionActive` and a message containing the session name

#### Scenario: No active session
- **WHEN** `QueryServerState` returns an empty `activeSessionName`
- **THEN** `WorldLoaded` SHALL be `False` with reason `NoSession`

### Requirement: Populate ConnectedPlayers from game API
The reconciler SHALL set `status.connectedPlayers` from the `numConnectedPlayers` field returned by `QueryServerState`.

#### Scenario: Players connected
- **WHEN** `QueryServerState` returns `numConnectedPlayers=3`
- **THEN** `status.connectedPlayers` SHALL be `3`

#### Scenario: Server unreachable
- **WHEN** the game server API is unreachable
- **THEN** `status.connectedPlayers` SHALL be `0`

### Requirement: Populate ServerVersion from game API
The reconciler SHALL set `status.serverVersion` from the server build version returned by the game API.

#### Scenario: Version available
- **WHEN** `QueryServerState` returns server version information
- **THEN** `status.serverVersion` SHALL contain the version string

### Requirement: Single status update per reconciliation
The reconciler SHALL perform at most one `Status().Update()` call per reconciliation pass, after all conditions and status fields have been computed.

#### Scenario: All conditions computed before write
- **WHEN** the reconciler computes conditions for Progressing, ServerReachable, ServerClaimed, WorldLoaded, Degraded, and Ready
- **THEN** all conditions SHALL be set on the status object before the single `Status().Update()` call

### Requirement: SaveGame basic conditions
The SaveGame reconciler SHALL set a `Ready` condition to `True` when the SaveGame resource has been successfully reconciled, and `BackupScheduled` to `True` when a cron schedule is configured.

#### Scenario: SaveGame reconciled
- **WHEN** the SaveGame resource is reconciled without errors
- **THEN** the `Ready` condition SHALL be `True` with reason `Reconciled`

#### Scenario: Backup schedule configured
- **WHEN** the SaveGame spec includes a non-empty `schedule` field
- **THEN** the `BackupScheduled` condition SHALL be `True` with reason `ScheduleActive`

### Requirement: ClusterPolicy basic conditions
The ClusterPolicy reconciler SHALL set a `Ready` condition to `True` when the policy has been applied, and `Enforced` to `True` when all governed servers comply.

#### Scenario: Policy applied
- **WHEN** the ClusterPolicy resource is reconciled without errors
- **THEN** the `Ready` condition SHALL be `True` with reason `PolicyApplied`

#### Scenario: All servers compliant
- **WHEN** all SatisfactoryServer resources governed by the policy meet its constraints
- **THEN** the `Enforced` condition SHALL be `True` with reason `AllServersCompliant`

### Requirement: Short requeue when server unreachable
The reconciler SHALL requeue with a 5-second delay when `ServerReachable` is `False` to quickly detect when the game server becomes available.

#### Scenario: Requeue on unreachable
- **WHEN** the game server API probe fails
- **THEN** the reconciler SHALL return `ctrl.Result{RequeueAfter: 5 * time.Second}`

#### Scenario: Normal requeue when reachable
- **WHEN** the game server API probe succeeds
- **THEN** the reconciler SHALL return the default result (no explicit short requeue)

### Requirement: Authenticated condition for SatisfactoryServer
The system SHALL define an `Authenticated` condition type constant for SatisfactoryServer that indicates whether the controller holds a valid API token for the game server.

#### Scenario: Authenticated condition constant exists
- **WHEN** the `api/v1alpha1` package is compiled
- **THEN** a string constant `SatisfactoryServerConditionTypeAuthenticated = "Authenticated"` SHALL exist

#### Scenario: Authenticated is True after successful login
- **WHEN** the controller successfully authenticates via `PasswordLogin` or verifies a cached token
- **THEN** the `Authenticated` condition SHALL be `True` with reason `TokenValid`

#### Scenario: Authenticated is False when Secret not found
- **WHEN** the `adminPasswordSecretRef` references a nonexistent Secret
- **THEN** the `Authenticated` condition SHALL be `False` with reason `SecretNotFound`

#### Scenario: Authenticated is False when login fails
- **WHEN** `PasswordLogin` returns an authentication error
- **THEN** the `Authenticated` condition SHALL be `False` with reason `AuthenticationFailed`

#### Scenario: Authenticated is Unknown when no credentials configured
- **WHEN** `adminPasswordSecretRef` is not set on the SatisfactoryServer resource
- **THEN** the `Authenticated` condition SHALL not be set (absent from conditions array)
