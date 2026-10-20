## Context

The SatisfactoryServer, SaveGame, and ClusterPolicy CRDs all declare a `Conditions []metav1.Condition` field in their status structs, but no controller ever populates them. The SatisfactoryServer reconciler only sets a `Phase` field based on StatefulSet readiness. The `pkg/satisfactory` HTTP API client already supports `QueryServerState()` and `EnumerateSessions()` but is not called from any controller. Users have no way to use `kubectl wait --for=condition=...` or integrate with GitOps health checks.

## Goals / Non-Goals

**Goals:**
- Define a clear set of condition types for each CRD and populate them during reconciliation.
- Integrate the game server API client into the SatisfactoryServer reconciler to detect live server state.
- Populate the existing but empty status fields (`ServerVersion`, `ConnectedPlayers`, `LastSaveTime`).
- Provide a shared condition helper package so all three controllers set conditions consistently.

**Non-Goals:**
- Changing the `Phase` enum or phase-based status model — conditions supplement phases, they don't replace them.
- Adding webhook validation or defaulting.
- Implementing the full SaveGame or ClusterPolicy reconciliation logic — only their condition management.
- Exposing conditions as Prometheus metrics (future work).
- Adding readiness/liveness probes to the game server pods based on conditions.

## Decisions

### 1. Condition types per CRD

**SatisfactoryServer:**
| Type | Meaning |
|------|---------|
| `Ready` | Aggregated: StatefulSet ready AND server reachable AND world loaded |
| `Progressing` | StatefulSet rollout in progress (updated != ready replicas) |
| `Degraded` | StatefulSet ready but game server API unreachable or returning errors |
| `ServerReachable` | Game server HTTPS API responds to `HealthCheck` or `QueryServerState` |
| `ServerClaimed` | Server has been claimed (not in initial unclaimed state) |
| `WorldLoaded` | An active game session is loaded (`activeSessionName` is non-empty) |

**SaveGame:**
| Type | Meaning |
|------|---------|
| `Ready` | SaveGame resource is reconciled and operational |
| `BackupScheduled` | Cron schedule is active and next backup is planned |

**ClusterPolicy:**
| Type | Meaning |
|------|---------|
| `Ready` | Policy has been applied to all governed servers |
| `Enforced` | All governed servers comply with the policy |

**Rationale:** Standard types (`Ready`, `Progressing`, `Degraded`) follow the [Kubernetes API conventions](https://github.com/kubernetes/community/blob/master/contributors/dede/sig-architecture/api-conventions.md#typical-status-properties). Game-specific types provide domain insight without overloading standard semantics.

**Alternative considered:** Using only standard types and encoding game state into `message` strings. Rejected because it defeats programmatic consumption via `kubectl wait` and GitOps tools.

### 2. Condition helper package at `internal/conditions/`

A small package providing:
- `Set(obj client.Object, condType string, status metav1.ConditionStatus, reason, message string)` — sets or updates a condition on the object's status conditions slice.
- `Remove(obj client.Object, condType string)` — removes a condition.
- `IsTrue(obj client.Object, condType string) bool` — checks if a condition is True.

Uses `meta.SetStatusCondition` / `meta.FindStatusCondition` from `k8s.io/apimachinery/pkg/api/meta` under the hood, adding type-safe wrappers for each CRD.

**Rationale:** Centralises `ObservedGeneration` stamping and `LastTransitionTime` management. Prevents divergence between controllers.

**Alternative considered:** Inline condition setting in each controller. Rejected because three controllers would duplicate the same boilerplate.

### 3. Game server health probing strategy

The reconciler calls `QueryServerState()` on every reconciliation cycle after confirming the StatefulSet has ready replicas. The call targets the pod's headless service DNS name on the API port (default 7777 HTTPS).

- **Success** → set `ServerReachable=True`, populate `ConnectedPlayers`, extract session info.
- **Auth error (401/403)** → set `ServerReachable=True` (server is up), `ServerClaimed=False`.
- **Connection refused / timeout** → set `ServerReachable=False`, `Degraded=True`.
- **Active session present** → set `WorldLoaded=True`, populate `LastSaveTime` from session metadata.

The API client is instantiated per-reconciliation using the pod service URL. No persistent connections or caching — the reconciler is stateless.

**Rationale:** Keeps the reconciler idempotent. The game server API is lightweight (single HTTP request). Reconciliation interval (default ~10s on changes, periodic resync every 10m) is acceptable for status freshness.

**Alternative considered:** Running a separate goroutine/timer for health polling. Rejected — adds complexity and state; the standard requeue mechanism is sufficient.

### 4. Status update ordering

Within a single reconciliation pass:
1. Reconcile StatefulSet, Services (existing logic).
2. Update `Progressing` condition based on StatefulSet rollout status.
3. If StatefulSet has ready replicas, probe game server API.
4. Update game-specific conditions (`ServerReachable`, `ServerClaimed`, `WorldLoaded`).
5. Compute aggregate `Ready` condition.
6. Update `Phase` (existing logic).
7. Single `Status().Update()` call at the end.

**Rationale:** Single status write per reconciliation avoids conflicts and reduces API server load. Conditions are computed in dependency order so `Ready` can aggregate sub-conditions.

## Risks / Trade-offs

- **[Game API unavailable during startup]** → The server takes time to start its HTTPS API after the pod is ready. Mitigation: `ServerReachable=False` is expected during startup; `Ready` stays False until the API responds. Use a short requeue (5s) when `ServerReachable` is False.
- **[API auth token management]** → `QueryServerState` may require authentication after the server is claimed. Mitigation: Start with unauthenticated `HealthCheck`-style calls; defer full auth token management to a follow-up change. For now, treat auth failures as "server reachable but not yet integrated."
- **[Status update conflicts]** → If external actors also update status, `Update()` may conflict. Mitigation: Use `RetryOnConflict` wrapper around the status update. This is standard controller-runtime practice.
- **[Increased API server load]** → Adding conditions increases status payload size. Mitigation: Negligible — conditions are a handful of small structs. No new watches or informers needed.
