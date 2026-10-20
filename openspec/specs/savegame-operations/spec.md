# savegame-operations Specification

## Purpose
Perform save game operations against the game server API: triggered and cron-scheduled saves, retention-based cleanup and save discovery for status.

## Requirements
### Requirement: Trigger save via game API
The SaveGame controller SHALL call the `SaveGame` API endpoint to create a save file on the game server when a backup is due.

#### Scenario: Scheduled backup triggered
- **WHEN** the current time exceeds the next scheduled backup time (computed from `schedule` and `lastBackupTime`)
- **THEN** the controller SHALL call `SaveGame("<saveName>_<UTC timestamp YYYYMMDD-HHMMSS>")` on the authenticated game API client (the API overwrites saves with an identical name, so each backup needs a unique name for retention to work)

#### Scenario: Save succeeds
- **WHEN** `SaveGame` returns successfully
- **THEN** the controller SHALL update `status.lastBackupTime` to the current time and set `Phase=Active`

#### Scenario: Save fails
- **WHEN** `SaveGame` returns an error
- **THEN** the controller SHALL set `Phase=Failed`, set `Ready=False` with reason `SaveFailed`, and requeue for retry at the next scheduled time

### Requirement: Cron-based backup scheduling
The SaveGame controller SHALL compute the next backup time from the `schedule` cron expression and `status.lastBackupTime`, and requeue at the appropriate interval.

#### Scenario: Schedule configured with no prior backup
- **WHEN** a SaveGame has `schedule: "0 * * * *"` and `lastBackupTime` is nil
- **THEN** the controller SHALL trigger an immediate backup and set the requeue interval to the next cron tick

#### Scenario: Schedule configured with prior backup
- **WHEN** a SaveGame has `schedule: "0 * * * *"` and `lastBackupTime` is 30 minutes ago
- **THEN** the controller SHALL requeue with `RequeueAfter: 30m` (next hourly tick)

#### Scenario: No schedule configured
- **WHEN** a SaveGame has an empty `schedule` field
- **THEN** the controller SHALL trigger a single one-shot save if `lastBackupTime` is nil, set `Phase=Completed`, and SHALL NOT requeue for time-based triggers or trigger further backups

### Requirement: Retention policy enforcement
After each successful backup, the controller SHALL enforce the retention policy by deleting excess saves via the game API.

#### Scenario: maxCount exceeded
- **WHEN** a backup completes and `retentionPolicy.maxCount` is 5 and there are 7 saves for the session
- **THEN** the controller SHALL call `DeleteSaveFile` for the 2 oldest saves

#### Scenario: maxAge exceeded
- **WHEN** a backup completes and `retentionPolicy.maxAge` is `"72h"` and there are saves older than 72 hours
- **THEN** the controller SHALL call `DeleteSaveFile` for each save whose `saveDateTime` exceeds the max age

#### Scenario: No retention policy
- **WHEN** `retentionPolicy` is nil
- **THEN** the controller SHALL NOT delete any saves

### Requirement: Enumerate sessions for status
The SaveGame controller SHALL call `EnumerateSessions` to discover existing saves and populate status fields.

#### Scenario: Saves found for session
- **WHEN** `EnumerateSessions` returns saves named `saveName` or `saveName_*`
- **THEN** the controller SHALL set `status.retainedSaveCount` to their number. `status.backupSize` is left unset because `SaveHeader` carries no file size

#### Scenario: Server not reachable
- **WHEN** the referenced SatisfactoryServer does not have `ServerReachable=True`
- **THEN** the controller SHALL set `Phase=Pending` and requeue with a short delay

### Requirement: Authenticated client from referenced server
The SaveGame controller SHALL resolve the SatisfactoryServer referenced by `serverRef`, read its `adminPasswordSecretRef`, and build an authenticated `pkg/satisfactory.Client`.

#### Scenario: Server reference valid and authenticated
- **WHEN** the referenced SatisfactoryServer exists and has a valid `adminPasswordSecretRef`
- **THEN** the controller SHALL obtain an authenticated client and proceed with API calls

#### Scenario: Server reference not found
- **WHEN** `serverRef` references a SatisfactoryServer that does not exist
- **THEN** the controller SHALL set `Phase=Failed`, `Ready=False` with reason `ServerNotFound`, and requeue

#### Scenario: Server not authenticated
- **WHEN** the referenced SatisfactoryServer has no `adminPasswordSecretRef` or authentication fails
- **THEN** the controller SHALL set `Phase=Pending`, `Ready=False` with reason `ServerNotAuthenticated`, and requeue
