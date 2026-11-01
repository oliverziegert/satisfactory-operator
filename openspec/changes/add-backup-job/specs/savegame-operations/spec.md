# Spec Delta

## MODIFIED Requirements

### Requirement: Trigger save via game API
When a backup is due, the SaveGame controller SHALL create a backup Job (see `backup-destinations`). The Job SHALL call the `SaveGame` API endpoint to create the save file on the game server.

#### Scenario: Scheduled backup triggered
- **WHEN** the current time exceeds the next scheduled backup time (computed from `schedule` and `lastBackupTime`)
- **THEN** the controller SHALL create a backup Job for the save name `<saveName>_<UTC timestamp YYYYMMDD-HHMMSS>`, and the Job SHALL call `SaveGame` with that name (the API overwrites saves with an identical name, so each backup needs a unique name for retention to work)

#### Scenario: Save succeeds
- **WHEN** the Job reports `saved: true`
- **THEN** the controller SHALL update `status.lastBackupTime` to the run's time, and SHALL set `Phase=Active` if every destination succeeded

#### Scenario: Save fails
- **WHEN** the Job reports `saved: false`, or failed without a result
- **THEN** the controller SHALL set `Phase=Failed`, set `Ready=False` with reason `SaveFailed` (or `BackupJobFailed` without a result), and requeue for retry at the next scheduled time

### Requirement: Retention policy enforcement
After each successful write to a destination, the backup Job SHALL enforce that destination's retention policy (its own `retentionPolicy`, or else `spec.retentionPolicy`) by deleting excess saves from that destination.

#### Scenario: maxCount exceeded
- **WHEN** a backup completes and the effective `maxCount` for a destination is 5 and it holds 7 saves for the session
- **THEN** the Job SHALL delete the 2 oldest saves from that destination

#### Scenario: maxAge exceeded
- **WHEN** a backup completes and the effective `maxAge` for a destination is `"72h"` and it holds saves older than 72 hours
- **THEN** the Job SHALL delete each of those saves from that destination

#### Scenario: No retention policy
- **WHEN** neither the destination nor the spec has a `retentionPolicy`
- **THEN** the Job SHALL NOT delete any saves from that destination

### Requirement: Enumerate sessions for status
The SaveGame controller SHALL take retained counts from the backup Job's result. Between runs it SHALL NOT call the game API to count saves.

#### Scenario: Saves found for session
- **WHEN** a backup Job reports per-destination retained counts
- **THEN** the controller SHALL set `status.destinations[].retainedCount` from them, and SHALL set `status.retainedSaveCount` to the count of the `server` destination, or 0 if there is none

#### Scenario: Server not reachable
- **WHEN** the referenced SatisfactoryServer does not have `ServerReachable=True`
- **THEN** the controller SHALL set `Phase=Pending`, create no Job, and requeue with a short delay
