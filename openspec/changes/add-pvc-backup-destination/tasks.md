# Tasks

## 1. API

- [ ] 1.1 Add `PVC *PVCDestination` (`claimName`, `path`) to `SaveGameDestination`, include it in the "exactly one type" CEL rule, and add `path` rules (relative, no `..`, at most 256 characters). Run `make manifests generate helm`. Verify with envtest cases for a valid path, an absolute path, `a/../b`, and `pvc` combined with `server` in one entry.

## 2. Destination

- [ ] 2.1 Implement the `pvc` destination on a root directory: `Put` (tmp, fsync, read-back hash check, rename, directory fsync), `List` (only `.sav`, `ModTime` from the name with mtime as fallback, removes `.tmp` older than 24h), `Delete`, `Get`. Verify with unit tests in `t.TempDir()`: round trip, a leftover `.tmp` is ignored, a hash mismatch fails and cleans up, `MkdirAll` for `path`.

## 3. Job and controller

- [ ] 3.1 Add a volume and mount per `pvc` destination (`dest-<name>` at `/backup/<name>`), and set `fsGroup: 65532` with `fsGroupChangePolicy: OnRootMismatch` on the Job pod. Verify with the Job spec unit test.
- [ ] 3.2 Check that claims exist before creating the Job (`BackupVolumeNotFound`), and make sure the `persistentvolumeclaims` get RBAC marker is present on the SaveGame controller. Verify with envtest: missing claim gives no Job and the reason; after the claim is created, a Job appears.

## 4. End-to-end and docs

- [ ] 4.1 e2e: a SaveGame with a `pvc` destination (Kind's default storage class, RWO) produces a `.sav` file on the claim. Check it with a pod that mounts the claim and runs `ls`. Run `make test-e2e`.
- [ ] 4.2 README: the `pvc` example, the PVC ownership statement, the RWO/RWX guidance, and the `fsGroup` 65532 note for NFS. Verify that the YAML applies.

## Workflow follow-up

- Open the PR titled `feat: add pvc backup destination`.
- Archive the change after merge.
