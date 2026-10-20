## Why

The SaveGame controller is a stub that only sets conditions. It doesn't interact with the game server API to perform actual save operations, schedule backups, or enforce retention policies. The API client already has all save-related methods implemented (`SaveGame`, `EnumerateSessions`, `DownloadSaveGame`, `DeleteSaveFile`). Without a functional SaveGame controller, users have no Kubernetes-native way to manage game saves.

**Depends on:** `server-auth-secret-management` (needs authenticated API access to call save-related endpoints).

## What Changes

- Resolve the referenced `SatisfactoryServer` and obtain an authenticated API client using its `adminPasswordSecretRef`
- On reconciliation, call `EnumerateSessions` to discover existing saves and populate status fields (`lastBackupTime`, `backupSize`)
- Implement cron-based scheduling: compare `schedule` against current time and `lastBackupTime` to determine if a backup is due, call `SaveGame(saveName)`, and requeue at the next scheduled time
- Implement retention policy enforcement: after each backup, call `EnumerateSessions`, delete saves exceeding `maxCount` or `maxAge` via `DeleteSaveFile`
- Watch the referenced `SatisfactoryServer` resource so the SaveGame re-reconciles when server state changes (e.g., becomes reachable)
- Update Phase to reflect actual operational state (Active when schedule is running, Completed after one-shot, Failed on errors)

## Capabilities

### New Capabilities

- `savegame-operations`: The controller performs actual save operations via the game server API — triggered saves, cron-scheduled backups, and retention-based cleanup.

### Modified Capabilities

- `save-management`: The SaveGame controller moves from stub to functional. Phase tracking, conditions, and status fields now reflect real save state from the API.

## Impact

- `internal/controller/savegame_controller.go` — full Reconcile implementation replacing the stub
- `internal/controller/savegame_helpers.go` — new file with save operation, scheduling, and retention helpers
- `api/v1alpha1/savegame_types.go` — possible additions to status fields (e.g., `nextBackupTime`, `retainedSaveCount`)
- `config/crd/bases/` — regenerated if types change
