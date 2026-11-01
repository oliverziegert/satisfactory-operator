# Proposal

## Why

Saves never leave the game server today. `SaveGameReconciler` calls the game API's `SaveGame`, so every backup lives on the server's own `/config` PVC, the same volume it is meant to protect. Losing that PVC, or deleting the server with `retainPVCOnDelete: false`, loses every backup. Off-site backups need the save bytes copied somewhere else, and the operator needs one extension point that each new destination type (PVC, S3, later others) can implement.

This change builds that foundation without adding any new storage yet. It moves backup execution into a Job and introduces the `Destination` interface and the `spec.destinations` API, with one built-in type, `server`. The `pvc` and `s3` types follow in `add-pvc-backup-destination` and `add-s3-backup-destination`.

## What Changes

- Each backup run is a **Job in the SaveGame's namespace**. It runs the operator image with the subcommand `manager backup`. The manager learns its image from the env var `RELATED_IMAGE_BACKUP`.
- The Job runs the whole backup:
  1. trigger the save through the game API
  2. download the save once into an `emptyDir`
  3. `Put` the file to each destination
  4. apply retention on each destination
  5. delete the server-side copy if no `server` destination is listed
  6. report the per-destination results through the pod's termination message
- The reconciler creates at most **one Job at a time per SaveGame**, waits for it, and writes the result into status. It remains the only writer of SaveGame status. The Job's pod gets no Kubernetes API token.
- New `internal/backup` package with the `Destination` interface (`Put`, `List`, `Delete`, `Get`) and the `server` implementation.
- New field **`spec.destinations[]`**. Each entry has a `name` and exactly one type; only `server: {}` exists in this change. A missing or empty list means `[{name: server, server: {}}]`, which is today's behaviour.
- Each destination has an optional `retentionPolicy` that falls back to `spec.retentionPolicy`.
- New `status.destinations[]` with per-destination results. A failed destination sets `Ready=False` with reason `DestinationFailed`.
- RBAC: `batch/jobs` create/get/list/watch/delete. Pods are already readable.
- Kustomize and the Helm chart set `RELATED_IMAGE_BACKUP` from the same value as the manager image.

## Capabilities

### New Capabilities
- `backup-destinations`: the backup Job, the `Destination` interface, the `spec.destinations` API, per-destination retention and status, and the `server` destination.

### Modified Capabilities
- `savegame-operations`: saving and retention now run inside the backup Job instead of the reconciler.
- `save-management`: new spec and status fields, Job ownership and RBAC.
- `helm-chart`: the chart sets the backup image.

## Impact

- `api/v1alpha1/savegame_types.go`: `Destinations`, `SaveGameDestination`, `ServerDestination`, `SaveGameDestinationStatus`, and CEL rules. Run `make manifests generate helm`.
- `internal/backup/` (new): the interface, the `server` implementation, the run logic and the result type.
- `cmd/main.go`: the `backup` subcommand next to the manager.
- `internal/controller/savegame_controller.go` and `savegame_helpers.go`: Job creation and observation. Retention and save code moves to `internal/backup`.
- `config/manager/`: the `RELATED_IMAGE_BACKUP` env var plus a kustomize `replacements` entry. `Makefile` `run` target sets the env var.
- `dist/chart/`: the env var, set from the image values.
- Behaviour with an empty `destinations` list is the same as today, except the save is now made by a Job. The first run after upgrading therefore needs the operator image to be pullable in the SaveGame's namespace.
- No new Go dependencies.
