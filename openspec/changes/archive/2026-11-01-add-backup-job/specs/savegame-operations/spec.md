# Spec Delta

## MODIFIED Requirements

### Requirement: Trigger save via game API
When a backup is due, the SaveGame controller SHALL create a backup Job (see `backup-destinations`). The Job SHALL call the `SaveGame` API endpoint to create the save file on the game server.

#### Scenario: Scheduled backup triggered
- **WHEN** the next backup time (see Cron-based backup scheduling) has passed
- **THEN** the controller SHALL create a backup Job for the save name `<saveName>_<UTC timestamp YYYYMMDD-HHMMSS>`, and the Job SHALL call `SaveGame` with that name (the API overwrites saves with an identical name, so each backup needs a unique name for retention to work)

#### Scenario: Save succeeds
- **WHEN** the Job reports `saved: true`
- **THEN** the controller SHALL update `status.lastBackupTime` to the run's time, and SHALL set `Phase=Active` if every destination succeeded

#### Scenario: Save fails
- **WHEN** the Job reports `saved: false`, or failed without a result
- **THEN** the controller SHALL set `Phase=Failed`, set `Ready=False` with reason `SaveFailed` (or `BackupJobFailed` without a result), and requeue for retry at the next scheduled time

### Requirement: Cron-based backup scheduling
The SaveGame controller SHALL compute the next backup time from the `schedule` cron expression and `status.nextBackupTime`, and requeue at the appropriate interval. When it creates a backup Job, it SHALL set `status.nextBackupTime` to the first cron tick after the run, so a failed run waits for that tick instead of starting a new Job. If the schedule's next tick is earlier than `status.nextBackupTime`, the earlier tick SHALL apply. Without `nextBackupTime`, the controller SHALL use the first tick after `lastBackupTime`, and without either it SHALL back up immediately.

#### Scenario: Schedule configured with no prior backup
- **WHEN** a SaveGame has `schedule: "0 * * * *"` and neither `nextBackupTime` nor `lastBackupTime` is set
- **THEN** the controller SHALL create a backup Job immediately and set `nextBackupTime` to the next cron tick

#### Scenario: Schedule configured with prior backup
- **WHEN** a SaveGame has `schedule: "0 * * * *"` and `nextBackupTime` is 30 minutes ahead
- **THEN** the controller SHALL create no Job and requeue with `RequeueAfter: 30m`

#### Scenario: Failed run
- **WHEN** a scheduled backup Job fails
- **THEN** the controller SHALL create no new Job before `nextBackupTime`

#### Scenario: Schedule moved earlier
- **WHEN** the `schedule` is changed so that its next tick is before `nextBackupTime`
- **THEN** the controller SHALL back up at the earlier tick

#### Scenario: No schedule configured
- **WHEN** a SaveGame has an empty `schedule` field
- **THEN** the controller SHALL create a single one-shot backup Job if `lastBackupTime` is nil, set `Phase=Completed` once it succeeds, and SHALL NOT trigger further backups. If that Job fails, the controller SHALL retry 30 seconds after the failed run.

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
