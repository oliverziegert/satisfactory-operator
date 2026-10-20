## 1. Condition Type Constants

- [x] 1.1 Define SatisfactoryServer condition type constants (`ConditionTypeReady`, `ConditionTypeProgressing`, `ConditionTypeDegraded`, `ConditionTypeServerReachable`, `ConditionTypeServerClaimed`, `ConditionTypeWorldLoaded`) in `api/v1alpha1/`
- [x] 1.2 Define SaveGame condition type constants (`ConditionTypeReady`, `ConditionTypeBackupScheduled`) in `api/v1alpha1/`
- [x] 1.3 Define ClusterPolicy condition type constants (`ConditionTypeReady`, `ConditionTypeEnforced`) in `api/v1alpha1/`

## 2. Condition Helper Package

- [x] 2.1 Create `internal/conditions/conditions.go` with `Set()`, `Remove()`, and `IsTrue()` helper functions wrapping `meta.SetStatusCondition` / `meta.FindStatusCondition`
- [x] 2.2 Add per-CRD typed wrappers that extract the `Conditions` slice from SatisfactoryServer, SaveGame, and ClusterPolicy status structs
- [x] 2.3 Write unit tests for the condition helper package

## 3. SatisfactoryServer Reconciler — Game API Integration

- [x] 3.1 Add game API client instantiation in the SatisfactoryServer reconciler (construct client using pod headless service DNS name)
- [x] 3.2 Add `probeGameServer()` method that calls `QueryServerState()` and returns structured probe results (reachable, claimed, session info, player count, version)
- [x] 3.3 Handle API response cases: success, auth error (401/403), connection failure/timeout

## 4. SatisfactoryServer Reconciler — Condition Updates

- [x] 4.1 Set `Progressing` condition based on StatefulSet `updatedReplicas` vs `readyReplicas`
- [x] 4.2 Set `ServerReachable` condition from game API probe result
- [x] 4.3 Set `ServerClaimed` condition from game API probe result
- [x] 4.4 Set `WorldLoaded` condition from game API probe result (active session name)
- [x] 4.5 Set `Degraded` condition when StatefulSet is ready but API probe fails
- [x] 4.6 Compute and set aggregate `Ready` condition (StatefulSet ready AND ServerReachable AND WorldLoaded)
- [x] 4.7 Populate `status.connectedPlayers` and `status.serverVersion` from probe results
- [x] 4.8 Consolidate to a single `Status().Update()` call at end of reconciliation
- [x] 4.9 Return short requeue (5s) when `ServerReachable` is False

## 5. SaveGame & ClusterPolicy Reconciler — Basic Conditions

- [x] 5.1 Set `Ready` condition in SaveGame reconciler on successful reconciliation
- [x] 5.2 Set `BackupScheduled` condition in SaveGame reconciler when schedule field is non-empty
- [x] 5.3 Set `Ready` condition in ClusterPolicy reconciler on successful reconciliation
- [x] 5.4 Set `Enforced` condition in ClusterPolicy reconciler (placeholder: True when reconciled)

## 6. Testing

- [x] 6.1 Write unit tests for `probeGameServer()` with mock HTTP responses (success, auth error, timeout)
- [x] 6.2 Write unit tests for SatisfactoryServer condition logic (all condition type transitions)
- [x] 6.3 Write unit tests for SaveGame and ClusterPolicy condition setting
- [x] 6.4 Verify CRD manifests regenerate cleanly with `make manifests generate`
