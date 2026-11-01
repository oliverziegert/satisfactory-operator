# Design

## Context

`SaveGameReconciler` handles a backup inline:
1. `resolveServerClient` authenticates against the game server, reusing the server controller's token cache in the `<server>-auth-token` Secret, key `token`.
2. `triggerSave` creates `<saveName>_<YYYYMMDD-HHMMSS>`.
3. `enumerateSessionSaves` and `enforceRetention` prune through `EnumerateSessions` and `DeleteSaveFile`.

The game API client also has `DownloadSaveGame` (a stream) and `UploadSaveGame`. The manager is one central pod, cluster-wide or limited to `--watch-namespaces`. In namespace mode, the Helm chart binds the generated `manager-role` ClusterRole per namespace, so new RBAC markers reach both modes without extra work. Pods are excluded from the cache and read directly.

The design decisions were made in a grilling session. The options that were rejected are recorded below.

## Goals / Non-Goals

**Goals:**
- One extension point (`Destination`) that `pvc` and `s3` can implement without changing the reconciler.
- No change in behaviour for SaveGames without `destinations`.
- Credentials and volumes stay in the SaveGame's namespace, so the operator needs no new cluster-wide Secret access.

**Non-Goals:**
- `pvc` and `s3` destinations (separate changes).
- Restore. `Get` is part of the interface so that restore can build on it, but there is no restore API or Job mode.
- A `BackupDestination` CRD, ClusterPolicy defaults or allowlists for destinations, encryption, compression.

## Decisions

- **One Job for every backup run, not the operator process.** The central pod can't mount PVCs in user namespaces, and having it stream S3 uploads would mean a second code path. The Job runs `manager backup` from the operator image, so the interface lives in the Job binary only. Rejected: operator-side streaming for S3 with Jobs only for PVCs (two failure models); a sidecar in the game server StatefulSet (changing a destination would restart the server).

- **The reconciler prepares and the Job executes.** Before creating a Job, the reconciler runs `resolveServerClient`, so the token is valid and cached, and it resolves the game API URL. The Job receives:
  - `--server-url`
  - `--save-name`: the timestamped name, picked by the reconciler so the Job is deterministic
  - `--session-prefix`: `spec.saveName`, used for retention matching
  - a JSON destination config with retention already resolved
  - the `<server>-auth-token` Secret, mounted read-only at `/var/run/satisfactory/token`

  The Job neither logs in nor reads the admin password.

- **Results come back through the termination message.** The Job writes a JSON result to `/dev/termination-log`, with `terminationMessagePolicy: FallbackToLogsOnError`:
  ```json
  {"saveName":"factory_20261101-120000","saved":true,"size":12345678,"sha256":"…",
   "destinations":[{"name":"server","ok":true,"retained":5},{"name":"offsite","ok":false,"error":"…"}]}
  ```
  It stays well below the 4 KiB limit for up to about 10 destinations, and error strings are truncated to 256 bytes. The reconciler reads it from the Job's pod, which it can already read. The Job pod runs with `automountServiceAccountToken: false`. Rejected: the Job patching status (it would need a Role per namespace and race the reconciler), and a ConfigMap (same RBAC cost).

- **One Job at a time per SaveGame.**
  - Jobs are named `<savegame>-<unix seconds of the run>` (truncated to the 63-character limit) and owned by the SaveGame. The controller adds `Owns(&batchv1.Job{})`.
  - If an unfinished owned Job exists, no new one is created.
  - Each Job gets `backoffLimit: 0`, because a retry would create a second save. A failure is a failed run, and the next schedule tick tries again.
  - `activeDeadlineSeconds` is 1800 and `ttlSecondsAfterFinished` is 3600. The result already lives in status, so finished Jobs aren't kept.

- **Download once, put many times.** The Job downloads the save once into an `emptyDir` (`sizeLimit: 2Gi`), computing SHA-256 on the way, and opens the file again for each `Put`. The game server is asked for the bytes once per run, whatever the number of destinations.

- **The `Destination` interface** (`internal/backup`):
  ```go
  type Object struct { Name string; Size int64; ModTime time.Time }
  type Destination interface {
      Put(ctx context.Context, name string, r io.Reader) error
      List(ctx context.Context, prefix string) ([]Object, error)
      Delete(ctx context.Context, name string) error
      Get(ctx context.Context, name string) (io.ReadCloser, error)
  }
  ```
  - Object names are `<saveName>_<YYYYMMDD-HHMMSS>` plus a `.sav` extension for file stores. The `server` destination uses the bare save name.
  - Retention works on `List(<saveName>_)` sorted by name, which is chronological because the timestamp is zero-padded. The existing `enforceRetention` logic moves here and works on `[]Object`, keeping the rule "never delete what we cannot date".
  - Left out on purpose: `Stat`, metadata, tags. `List` covers existence checks.

- **The `server` destination:**
  - `Put` is a no-op, because the save is already on the server.
  - `List` uses `EnumerateSessions`, with `ModTime` from `saveDateTime` and `Size` 0, since the API returns none.
  - `Delete` uses `DeleteSaveFile`.
  - `Get` uses `DownloadSaveGame`.

- **The server copy is deleted only when it is safe.**
  - With no `server` entry, the Job deletes the current save from the server only after every destination's `Put` succeeded.
  - After such a run, it also deletes earlier `<saveName>_*` server saves that `List` finds on every destination. These are leftovers from failed runs.
  - If any `Put` failed, nothing on the server is deleted.
  - Server saves that never reached a destination stay, for example saves from before destinations were configured.

- **The API is inline, and the default is explicit in code.** `spec.destinations` is a list of `{name, retentionPolicy?, server?}`. Change 2 adds `pvc?` and change 3 adds `s3?`. CEL rules:
  - exactly one type field per entry
  - unique `name`s
  - at most one `server` entry
  - names must be DNS-label safe, because they become volume names in change 2

  An empty list is resolved to `[{name: server, server: {}}]` in code. It isn't a CRD default, so the stored object stays unchanged. Rejected: a `BackupDestination` CRD. It can be added later without breaking the inline form.

- **The Job image comes from `RELATED_IMAGE_BACKUP`.** The `RELATED_IMAGE_*` prefix is what OLM uses for disconnected installs.
  - Kustomize: a `replacements` entry copies the manager container's `image` into the env var, so `kustomize edit set image` (used by `make deploy`, `build-installer` and `bundle`) keeps both in sync.
  - Helm: the chart sets it from `manager.image.repository`/`tag`, using the same expression as the manager image.
  - `make run` sets it explicitly.
  - If it is missing, the SaveGame gets `Ready=False` with reason `BackupImageNotConfigured`, and the manager doesn't crash.

  Rejected: reading the manager's own Pod (extra RBAC, breaks `make run`), and a flag without a default.

- **Job pod hardening:**
  - `runAsNonRoot`, `readOnlyRootFilesystem`, all capabilities dropped, `seccompProfile: RuntimeDefault`
  - requests of 50m CPU and 64Mi memory, a 256Mi memory limit and no CPU limit
  - `imagePullPolicy: IfNotPresent` and no pull secrets. Configurable pull settings are skipped until a private mirror needs them.

## Risks / Trade-offs

- [The operator image must be pullable in every SaveGame namespace] → It's public on GHCR. Document it for private mirrors.
- [Pod startup adds seconds to every run] → Acceptable for a backup on a cron schedule.
- [The token can expire between the reconciler's check and the Job's API call] → The Job fails with an auth error, the run is reported failed, and the next tick refreshes the token first.
- [`backoffLimit: 0` turns transient failures into a failed run] → Retrying inside the Job (bounded, per destination) is cheaper and doesn't create a second save. The `Put` calls get 3 attempts with backoff.
- [Retention on a remote destination deletes an object that a later leftover cleanup would need] → The server copy then simply stays. That's the conservative failure mode.
- [Helm chart regeneration overwrites `templates/manager/manager.yaml`] → Check whether the `helm/v2-alpha` plugin carries env from `config/manager` into templated values, the way it does for `args`. If it doesn't, the `make helm` post-processing step injects the env var, like it already does for the rolebinding wrapper.
