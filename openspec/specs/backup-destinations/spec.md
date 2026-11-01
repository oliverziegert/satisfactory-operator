# backup-destinations Specification

## Purpose
Copy save games to one or more backup destinations through a common interface, so that backups can live somewhere other than the game server's own volume.

## Requirements
### Requirement: Destinations list
The `SaveGameSpec` SHALL have an optional `destinations` list of at most 10 entries, which keeps the Job result within the termination message limit. Each entry SHALL have a `name`, an optional `retentionPolicy` and exactly one destination type field. The API server SHALL reject a SaveGame in which an entry has zero or more than one type field, two entries share a `name`, more than one entry has the `server` type, or a `name` is not a valid DNS label.

#### Scenario: Valid list
- **WHEN** a SaveGame is applied with `destinations: [{name: local, server: {}}]`
- **THEN** the API server accepts it

#### Scenario: Too many entries
- **WHEN** a SaveGame is applied with 11 destination entries
- **THEN** the API server rejects the SaveGame

#### Scenario: Two types in one entry
- **WHEN** a destination entry sets two type fields
- **THEN** the API server rejects the SaveGame

#### Scenario: Duplicate server destination
- **WHEN** two entries both set `server: {}`
- **THEN** the API server rejects the SaveGame

#### Scenario: Duplicate names
- **WHEN** two entries share the name `offsite`
- **THEN** the API server rejects the SaveGame

### Requirement: Default destination
When `spec.destinations` is missing or empty, the operator SHALL behave as if it were `[{name: server, server: {}}]`. It SHALL NOT write that default into the stored object.

#### Scenario: Existing SaveGame without destinations
- **WHEN** a SaveGame without `destinations` runs a backup
- **THEN** the save stays on the game server, server-side retention follows `spec.retentionPolicy`, and `status.destinations` has one entry named `server`

### Requirement: Server destination
The `server` destination type SHALL keep the save on the game server. Listing SHALL use `EnumerateSessions`, deleting SHALL use `DeleteSaveFile`, and reading SHALL use `DownloadSaveGame`. Writing SHALL be a no-op, because the game API save already created the file.

#### Scenario: Server destination listed
- **WHEN** a run completes with a `server` destination
- **THEN** the new save remains on the game server

### Requirement: Backup Job per run
For each due backup, the operator SHALL create a Job in the SaveGame's namespace, owned by the SaveGame, that runs the operator image with the `backup` subcommand. While an owned Job of a SaveGame has not finished, the operator SHALL NOT create another one for that SaveGame. The Job SHALL use `backoffLimit: 0`, an `activeDeadlineSeconds`, and a `ttlSecondsAfterFinished`.

#### Scenario: Backup due
- **WHEN** a backup is due and no owned Job is running
- **THEN** the operator creates one Job and does not trigger a save itself

#### Scenario: Previous run still active
- **WHEN** a backup becomes due while the previous Job is still running
- **THEN** the operator creates no new Job and re-checks after the running Job finishes

#### Scenario: Job exceeds deadline
- **WHEN** a Job exceeds `activeDeadlineSeconds`
- **THEN** the run is reported as failed and the next schedule tick creates a new Job

### Requirement: Backup Job pod hardening
The backup Job's pod SHALL NOT mount a Kubernetes API token. It SHALL run as non-root with a read-only root filesystem and all capabilities dropped.

#### Scenario: Pod spec
- **WHEN** the operator creates a backup Job
- **THEN** its pod has `automountServiceAccountToken: false`, `runAsNonRoot: true`, `readOnlyRootFilesystem: true` and drops all capabilities

### Requirement: Backup run sequence
The backup Job SHALL:
1. trigger a save named `<saveName>_<UTC YYYYMMDD-HHMMSS>` through the game API
2. download it once to local scratch storage while computing its SHA-256
3. write it to every configured destination
4. enforce each destination's retention policy

The Job SHALL authenticate with the token from the server's `<server>-auth-token` Secret and SHALL NOT read the admin password.

#### Scenario: Two destinations
- **WHEN** a run has destinations `server` and `nas`
- **THEN** the save is downloaded from the game server exactly once and written to `nas`

#### Scenario: Save API call fails
- **WHEN** the game API save call fails
- **THEN** the Job writes no destination, and the run is reported as failed with `saved: false`

### Requirement: Per-destination retention
Each destination SHALL apply its own `retentionPolicy`, or `spec.retentionPolicy` if it has none, after its write succeeded. Retention SHALL consider only objects whose name starts with `<saveName>_`. It SHALL delete the oldest objects beyond `maxCount` and objects older than `maxAge`. It SHALL keep objects whose age it cannot determine.

#### Scenario: Different policies
- **WHEN** `server` has `maxCount: 3`, `nas` has no policy, and `spec.retentionPolicy.maxCount` is 30
- **THEN** after the run `server` keeps at most 3 saves and `nas` keeps at most 30

#### Scenario: Foreign files untouched
- **WHEN** a destination also holds files that don't start with `<saveName>_`
- **THEN** retention leaves them alone

#### Scenario: Write failed
- **WHEN** writing to a destination fails
- **THEN** retention does not run on that destination in this run

### Requirement: Server copy removal
When no `server` destination is configured, the Job SHALL delete the run's save from the game server only after every destination write succeeded. After such a run, it SHALL also delete older `<saveName>_*` saves from the game server that are present on every configured destination. If any destination write failed, the Job SHALL NOT delete anything from the game server.

#### Scenario: All uploads succeed
- **WHEN** destinations are only `nas` and both the write and retention succeed
- **THEN** the run's save is deleted from the game server

#### Scenario: One upload fails
- **WHEN** destinations are `nas` and `offsite`, and the write to `offsite` fails
- **THEN** the run's save stays on the game server

#### Scenario: Leftover from an earlier failed run
- **WHEN** a save from an earlier failed run is on the game server and, after a successful run, is present on every destination
- **THEN** that save is deleted from the game server

#### Scenario: Save that never reached a destination
- **WHEN** a `<saveName>_*` save on the game server is missing on at least one destination
- **THEN** it stays on the game server

### Requirement: Run result reporting
The Job SHALL report its result as JSON in its termination message: the save name, whether the save succeeded, the size, the SHA-256, and for each destination whether it succeeded, the retained count and an error message truncated to 256 bytes.

#### Scenario: Result written
- **WHEN** a backup Job finishes, successfully or not, without crashing
- **THEN** its container's termination message holds the JSON result with one entry per configured destination

### Requirement: Destination status
The operator SHALL read the result from the finished Job's pod and SHALL be the only writer of SaveGame status. It SHALL set `status.destinations[]` entries with `name`, `lastSuccessTime`, `lastSize`, `retainedCount` and `lastError`. If any destination failed, it SHALL set `Ready=False` with reason `DestinationFailed` and `Phase=Failed`.

#### Scenario: All destinations succeed
- **WHEN** a Job completes and every destination reports success
- **THEN** `status.lastBackupTime` and `status.backupSize` are set, each `status.destinations[]` entry has a fresh `lastSuccessTime` and an empty `lastError`, and `Ready=True`

#### Scenario: One destination fails
- **WHEN** a Job reports `offsite` as failed
- **THEN** `status.destinations[offsite].lastError` holds the message, its `lastSuccessTime` is unchanged, and `Ready=False` with reason `DestinationFailed`

#### Scenario: Pod crashed without a result
- **WHEN** the Job failed and its termination message is not valid JSON
- **THEN** `Ready=False` with reason `BackupJobFailed` and a message taken from the pod's termination message or log tail

### Requirement: Backup image configuration
The operator SHALL take the backup Job image from the `RELATED_IMAGE_BACKUP` environment variable. When it is not set, the operator SHALL NOT create Jobs and SHALL set `Ready=False` with reason `BackupImageNotConfigured` on SaveGames that have a backup due.

#### Scenario: Env var missing
- **WHEN** the manager runs without `RELATED_IMAGE_BACKUP` and a backup is due
- **THEN** no Job is created and the SaveGame shows `Ready=False` with reason `BackupImageNotConfigured`

#### Scenario: Kustomize image override
- **WHEN** `kustomize edit set image controller=example.com/op:v9` is run and the manifests are built
- **THEN** both the manager container image and `RELATED_IMAGE_BACKUP` are `example.com/op:v9`

### Requirement: PVC destination
The `pvc` destination type SHALL write saves as `<saveName>_<timestamp>.sav` files into an existing, user-owned PersistentVolumeClaim in the SaveGame's namespace, under an optional relative `path`. The operator SHALL NOT create, modify or delete the claim. The API server SHALL reject a `path` that is absolute or contains a `..` segment, and a `pvc` destination name longer than 58 characters (the Job volume is named `dest-<name>`).

#### Scenario: Save written to the volume
- **WHEN** a run has destination `{name: nas, pvc: {claimName: backups, path: factory}}`
- **THEN** the claim `backups` contains `factory/<saveName>_<timestamp>.sav`, with the same SHA-256 as the downloaded save

#### Scenario: Invalid path
- **WHEN** a SaveGame sets `pvc.path: ../other`
- **THEN** the API server rejects it

#### Scenario: Destination name too long
- **WHEN** a SaveGame has a `pvc` destination whose name is 59 characters long
- **THEN** the API server rejects it

#### Scenario: SaveGame deleted
- **WHEN** a SaveGame with a `pvc` destination is deleted
- **THEN** the claim and its files remain

### Requirement: PVC save name containment
The `pvc` destination SHALL refuse save names that contain a path separator or `..`, so that a save never lands outside the destination directory.

#### Scenario: Save name with a separator
- **WHEN** a run writes the save name `../evil` to a `pvc` destination
- **THEN** the write fails and no file is created outside the destination directory

### Requirement: Atomic PVC writes
The `pvc` destination SHALL write to a temporary file, sync it, verify its SHA-256 against the downloaded save, and only then rename it to its final `.sav` name. Listing and retention SHALL ignore temporary files.

#### Scenario: Job killed during write
- **WHEN** the Job is terminated while writing to a `pvc` destination
- **THEN** no new `.sav` file is visible, and retention and status counts are unaffected

#### Scenario: Checksum mismatch
- **WHEN** the file read back from the volume has a different SHA-256 than the download
- **THEN** the write counts as failed, and the temporary file is removed

### Requirement: PVC existence check
Before creating a backup Job, the operator SHALL verify that every claim referenced by a `pvc` destination exists. If one doesn't, it SHALL create no Job and SHALL set `Ready=False` with reason `BackupVolumeNotFound`, naming the claim.

#### Scenario: Claim missing
- **WHEN** a backup is due and the claim `backups` doesn't exist
- **THEN** no Job is created and the SaveGame shows `Ready=False` with reason `BackupVolumeNotFound`

#### Scenario: Claim created later
- **WHEN** the missing claim is created afterwards
- **THEN** the next reconcile creates the backup Job
