# Proposal

## Why

With `add-backup-job`, a save can leave the game server, but the only destination is still the server itself. A PVC destination copies saves to a volume the user owns, such as NFS, Longhorn or another storage class. That volume survives deleting the server and its `/config` PVC, and you can browse or copy from it outside the game.

## What Changes

- New destination type `pvc: {claimName, path}`. `path` is an optional subdirectory, relative and without `..`.
- The PVC is **owned by the user** and must already exist in the SaveGame's namespace. The operator never creates or deletes it.
- The backup Job mounts each `pvc` destination's claim at `/backup/<destination name>`. A file is written as `<path>/<saveName>_<ts>.sav.tmp`, fsynced, its SHA-256 checked against the download, and then renamed to `.sav`.
- Before creating a Job, the reconciler checks that each referenced PVC exists. If one doesn't, it sets `Ready=False` with reason `BackupVolumeNotFound`.
- The Job pod gets `fsGroup: 65532` with `fsGroupChangePolicy: OnRootMismatch`, so the non-root Job user can write to fresh volumes.
- Documentation: RWO volumes block Jobs scheduled on another node, so use RWX, or one PVC per SaveGame.

## Capabilities

### Modified Capabilities
- `backup-destinations`: adds the `pvc` destination type.

## Impact

- `api/v1alpha1/savegame_types.go`: `PVC *PVCDestination` on `SaveGameDestination`, with CEL rules for `path`. Run `make manifests generate helm`.
- `internal/backup/`: `pvc` destination (`os` file operations).
- `internal/controller/`: PVC volumes and mounts in the Job spec, the PVC existence check, and RBAC `persistentvolumeclaims` get (the server controller already has it, so check the marker).
- README: the `pvc` example and the RWO/RWX note.
- Depends on `add-backup-job`. No new dependencies.
