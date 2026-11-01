# Tasks

## 1. API

- [x] 1.1 Add `Destinations []SaveGameDestination` (`name`, `retentionPolicy`, `server *ServerDestination`) to `SaveGameSpec`. Add `Destinations []SaveGameDestinationStatus` (`name`, `lastSuccessTime`, `lastSize`, `retainedCount`, `lastError`) to `SaveGameStatus`. Add CEL rules: exactly one type per entry, unique names, at most one `server` entry, DNS-label names. Run `make manifests generate`. Verify with envtest cases that create valid and invalid SaveGames (two types, duplicate names, two `server` entries).

## 2. `internal/backup`

- [x] 2.1 Define `Object` and `Destination`, plus a `resolve(spec)` that turns the spec into destinations with effective retention (the empty-list default goes here). Verify with a table test: empty list, inherited retention, overridden retention.
- [x] 2.2 Implement the `server` destination on `pkg/satisfactory.Client`: `Put` no-op, `List` via `EnumerateSessions`, `Delete` via `DeleteSaveFile`, `Get` via `DownloadSaveGame`. Verify against an `httptest.Server`, the same way `pkg/satisfactory/client_test.go` does.
- [x] 2.3 Move the retention logic (`enforceRetention`) from `savegame_helpers.go` to work on `[]Object` and keep the "never delete what we cannot date" rule. Port its existing tests.
- [x] 2.4 Implement the run: save, download to scratch with SHA-256, `Put` on each destination (3 attempts with backoff), retention on each destination that succeeded, server copy removal and leftover cleanup when there is no `server` destination, and the JSON result with errors truncated to 256 bytes. Verify with a unit test using a fake `Destination`: all succeed, one fails (no server delete), leftover present on all destinations or on only some, save call fails.

## 3. `backup` subcommand

- [x] 3.1 Add `backup` to `cmd/main.go` (`os.Args[1] == "backup"` before manager flag parsing). It takes `--server-url`, `--save-name`, `--session-prefix`, `--config` (JSON) and `--token-file`, and writes the result to `/dev/termination-log`. Exit code 0 only if the save and every destination succeeded. Verify with `go run ./cmd/main.go backup ...` against a fake server, or a test in `cmd/main_test.go`.

## 4. Controller

- [x] 4.1 Build the Job (name, owner reference, `backoffLimit: 0`, `activeDeadlineSeconds: 1800`, `ttlSecondsAfterFinished: 3600`, no service-account token, hardened security context, `emptyDir` with a 2Gi `sizeLimit`, token Secret mounted read-only, `terminationMessagePolicy: FallbackToLogsOnError`, resources). Verify with a plain unit test on the generated spec.
- [x] 4.2 Reconcile: if `RELATED_IMAGE_BACKUP` is empty, set `BackupImageNotConfigured`. When a backup is due and no owned Job is running, call `resolveServerClient`, then create the Job. For a finished Job, read its pod's termination message into status (`DestinationFailed`, `SaveFailed`, `BackupJobFailed`). Add `Owns(&batchv1.Job{})` and the `batch/jobs` RBAC marker, and remove the inline save, retention and `EnumerateSessions` calls. Verify with envtest: a Job is created when due; there's no second Job while one is running; status is filled from a simulated finished Job with a pod termination message; reason when the image is missing.
- [x] 4.3 Update the existing SaveGame envtest and unit tests that expected inline saves. Run `make test`.

## 5. Image wiring

- [x] 5.1 Add `RELATED_IMAGE_BACKUP` to `config/manager/manager.yaml` and a `replacements` entry in `config/manager/kustomization.yaml` that copies the manager image into it. Keep the `x-release-please-version` marker intact. Verify that `kustomize build config/default | grep -A1 RELATED_IMAGE_BACKUP` shows the image, also after `kustomize edit set image` (then restore the file, as `make deploy` does).
- [x] 5.2 Set `RELATED_IMAGE_BACKUP` in the `make run` target. Verify with `make run` and a SaveGame against a dev cluster (amd64 node, see the Talos test cluster).
- [x] 5.3 Run `make helm`, then check whether the generated manager template carries the env var templated from the image values. If it doesn't, extend the `make helm` post-processing to inject it. Verify with `helm template dist/chart --set manager.image.tag=v9.9.9 | grep -A1 RELATED_IMAGE_BACKUP` and a clean drift check (`make manifests generate helm && git status`).

## 6. End-to-end and docs

- [x] 6.1 Extend the e2e suite: a SaveGame without `destinations` produces a completed Job, a save on the server and `Ready=True`. Run `make test-e2e`.
- [x] 6.2 README: explain that backups run as Jobs in the SaveGame's namespace (the operator image must be pullable there), show the `destinations` field with `server`, and explain per-destination retention. Verify that the documented YAML applies.

## Workflow follow-up

- Open the PR titled `feat: run backups as Jobs with pluggable destinations`.
- Archive the change after merge.
