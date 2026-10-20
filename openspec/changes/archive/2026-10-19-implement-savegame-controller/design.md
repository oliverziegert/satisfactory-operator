## Context

The SaveGame CRD is defined with `serverRef`, `saveName`, `schedule` (cron), and `retentionPolicy` (maxCount, maxAge) fields. The controller currently sets `Ready=True` and `BackupScheduled` conditions but performs no API calls. The `pkg/satisfactory` client provides `SaveGame`, `EnumerateSessions`, `DeleteSaveFile`, and `DownloadSaveGame` — all requiring admin-level authentication.

The `server-auth-secret-management` change introduces the `adminPasswordSecretRef` field and token caching on SatisfactoryServer. The SaveGame controller needs to resolve the referenced server, read its auth credentials, and build an authenticated client.

## Goals / Non-Goals

**Goals:**

- Trigger game saves via the API (`SaveGame` endpoint)
- Schedule recurring backups using cron expressions
- Enforce retention policies (max count and max age)
- Populate status with real save metadata from `EnumerateSessions`
- Re-reconcile when the referenced server's state changes

**Non-Goals:**

- Downloading saves to external storage (S3, PVC) — future work
- Uploading saves to restore from backup — future work
- Cross-namespace server references
- Sub-minute scheduling precision

## Decisions

### 1. Time-based requeue for cron scheduling (no CronJob)

The controller computes the next trigger time from the cron expression and `lastBackupTime`, then returns `RequeueAfter: nextTrigger - now`. This avoids creating Kubernetes CronJob resources and keeps the implementation self-contained.

**Alternative considered:** Create a CronJob per SaveGame that calls the API. Rejected because it requires a separate container image, adds RBAC complexity, and duplicates the authentication logic.

### 2. Resolve server auth via SatisfactoryServer's adminPasswordSecretRef

The SaveGame controller reads the referenced SatisfactoryServer CR, extracts `adminPasswordSecretRef`, reads the password Secret, and authenticates. It reuses the `<server-name>-auth-token` cached token Secret when available.

**Alternative considered:** Add a separate `adminPasswordSecretRef` to SaveGameSpec. Rejected because it duplicates configuration and risks credential drift. The server ref already implies which credentials to use.

### 3. Retention enforced after each successful backup

After a successful `SaveGame` call, the controller calls `EnumerateSessions` to get all saves for the session, then deletes excess saves starting from the oldest. `maxCount` is applied first (delete oldest beyond count), then `maxAge` (delete anything older than the duration). Both are optional.

### 4. SaveGame watches SatisfactoryServer via field index

The controller sets up a field index on `spec.serverRef` and adds an `Watches` clause for SatisfactoryServer. When a server becomes reachable, all SaveGames referencing it are re-reconciled.

### 5. Phase semantics

- `Pending` — server not reachable or not authenticated
- `Active` — schedule is running, backups are being performed
- `Completed` — no schedule, last manual save succeeded
- `Failed` — last save or retention operation failed

## Risks / Trade-offs

**[Risk] Server becomes unreachable during save operation** → `SaveGame` API call will timeout. The controller sets `Phase=Failed`, keeps `BackupScheduled=True`, and retries at the next scheduled time.

**[Risk] Clock skew between controller and cron schedule** → Acceptable for backup scheduling where minute-level precision is sufficient. The controller uses `time.Now()` from the pod's clock.

**[Trade-off] No external backup storage** → Saves remain on the game server's filesystem only. External backup (download to PVC/S3) is deferred to a separate change to keep scope manageable.
