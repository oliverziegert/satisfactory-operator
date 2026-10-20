## 1. CRD Changes

- [x] 1.1 Add `nextBackupTime` (*metav1.Time) and `retainedSaveCount` (int32) to `SaveGameStatus` in `api/v1alpha1/savegame_types.go`
- [x] 1.2 Run `make manifests generate` to regenerate CRD manifests and deepcopy methods

## 2. Server Resolution and Authentication

- [x] 2.1 Add helper `resolveServerClient(ctx, serverRef, namespace) (*satisfactory.Client, error)` that fetches the SatisfactoryServer, reads its `adminPasswordSecretRef`, authenticates, and returns an authenticated client
- [x] 2.2 Add RBAC markers for reading SatisfactoryServer resources and Secrets from the SaveGame controller

## 3. Save Operations

- [x] 3.1 Add helper `triggerSave(ctx, client, saveName) error` that calls `client.SaveGame(ctx, saveName)`
- [x] 3.2 Add helper `enumerateSessionSaves(ctx, client, saveName) ([]SaveHeader, error)` that calls `EnumerateSessions` and filters to saves matching the saveName pattern
- [x] 3.3 Add helper `enforceRetention(ctx, client, saves, policy) error` that deletes saves exceeding maxCount or maxAge via `DeleteSaveFile`

## 4. Cron Scheduling

- [x] 4.1 Add cron expression parsing (use a lightweight cron library or `time`-based computation) to compute the next trigger time from `schedule` and `lastBackupTime`
- [x] 4.2 Integrate scheduling into Reconcile: compute next trigger, compare to `time.Now()`, trigger save if due, return `RequeueAfter` for next trigger

## 5. Controller Reconciliation

- [x] 5.1 Rewrite `SaveGameReconciler.Reconcile` with full logic: fetch SaveGame → resolve server → authenticate → check schedule → trigger save if due → enforce retention → enumerate for status → update conditions and phase → requeue
- [x] 5.2 Update `SetupWithManager` to watch SatisfactoryServer resources via field index on `spec.serverRef`
- [x] 5.3 Implement Phase derivation: Pending (server unreachable), Active (schedule running), Completed (one-shot done), Failed (error)

## 6. Tests

- [x] 6.1 Add unit tests for `resolveServerClient` — server found, server not found, auth failure
- [x] 6.2 Add unit tests for `triggerSave` and `enumerateSessionSaves` with mock HTTP server
- [x] 6.3 Add unit tests for `enforceRetention` — maxCount enforcement, maxAge enforcement, no policy
- [x] 6.4 Add unit tests for cron scheduling logic — immediate trigger, future trigger, no schedule
- [x] 6.5 Add integration tests for full Reconcile loop with envtest
- [x] 6.6 Run `make test` and verify all tests pass

## 7. Samples

- [x] 7.1 Update `config/samples/satisfactory_v1alpha1_savegame.yaml` with a realistic schedule and retention policy example