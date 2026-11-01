# Spec Delta

## ADDED Requirements

### Requirement: PVC destination
The `pvc` destination type SHALL write saves as `<saveName>_<timestamp>.sav` files into an existing, user-owned PersistentVolumeClaim in the SaveGame's namespace, under an optional relative `path`. The operator SHALL NOT create, modify or delete the claim. The API server SHALL reject a `path` that is absolute or contains a `..` segment.

#### Scenario: Save written to the volume
- **WHEN** a run has destination `{name: nas, pvc: {claimName: backups, path: factory}}`
- **THEN** the claim `backups` contains `factory/<saveName>_<timestamp>.sav`, with the same SHA-256 as the downloaded save

#### Scenario: Invalid path
- **WHEN** a SaveGame sets `pvc.path: ../other`
- **THEN** the API server rejects it

#### Scenario: SaveGame deleted
- **WHEN** a SaveGame with a `pvc` destination is deleted
- **THEN** the claim and its files remain

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
