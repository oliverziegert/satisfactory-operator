# Design

## Context

`add-backup-job` defines the `Destination` interface, the backup Job and the inline `spec.destinations` list. That change already restricts destination names to DNS labels, so they can be used as volume names. The Job runs as a non-root user with a read-only root filesystem.

## Goals / Non-Goals

**Goals:**
- Write saves to a user-owned PVC safely: no partially written files are ever visible under a `.sav` name.

**Non-Goals:**
- Provisioning PVCs (`storageClassName`, `size`). If the operator owned the volume, deleting a SaveGame would raise the question of whether its backups get deleted too.
- Coordinating access to shared RWO volumes across SaveGames.
- A configurable `fsGroup` (skipped until a storage class needs a different one).

## Decisions

- **User-owned PVC with an optional `path`.** CEL rules on `path`: it must be relative, must not contain `..` segments, and must be at most 256 characters. The Job creates `path` with `MkdirAll` if it's missing.
- **One mount per destination.** The volume name is `dest-<name>`, mounted at `/backup/<name>`. Two destinations may reference the same claim with different paths; Kubernetes then mounts the same claim twice in one pod, which is allowed. A volume name is a DNS label of at most 63 characters, so a CEL rule caps `pvc` destination names at 58.
- **Save names stay inside the directory.** `spec.saveName` is not validated, so the `pvc` destination rejects names that contain a path separator or `..`.
- **Atomic write.**
  1. Write to `<name>.sav.tmp`.
  2. `fsync` the file and close it.
  3. Read it back and compare SHA-256 with the downloaded file. A mismatch is a failed `Put`, and the `.tmp` file is removed.
  4. Rename it to `<name>.sav` and `fsync` the directory.

  `List` only returns `.sav` files, so a `.tmp` left by a crashed Job is never retained or counted. The next `Put` with the same name would overwrite it, which won't happen because names are timestamped. Stale `.tmp` files older than 24h are removed during `List`.
- **`ModTime` comes from the name.** It's parsed from the timestamp in the file name, falling back to the file's mtime, because copied or restored files may have a misleading mtime.
- **The existence check comes first.** The reconciler does a `Get` on each referenced PVC before creating a Job. If one is missing, it creates no Job and sets `Ready=False` with reason `BackupVolumeNotFound`, naming the claim. A PVC that exists but can't be attached shows up as a Job hitting `activeDeadlineSeconds`, and is reported as `BackupJobFailed`.
- **`fsGroup` is fixed at 65532** (the distroless `nonroot` user) with `fsGroupChangePolicy: OnRootMismatch`. NFS volumes that ignore `fsGroup` need suitable export permissions. That gets documented.

## Risks / Trade-offs

- [An RWO PVC already attached on another node keeps the Job `Pending`] → The deadline turns it into a failed run. The README recommends RWX, or one PVC per SaveGame.
- [The PVC is deleted while backups exist] → That's the user's decision. The operator only reports `BackupVolumeNotFound`.
- [Reading the file back for the hash doubles the I/O] → Saves are tens of MB, so it's negligible, and it catches write corruption on network filesystems.
