## Why

The operator currently only tracks a simple `Phase` field on each CRD status, with no meaningful use of the `Conditions` array already declared in the API types. This makes it impossible for users and automation (e.g., `kubectl wait --for=condition=Ready`) to observe granular operational state — whether the game server is actually reachable, whether saves are healthy, or whether a policy has been applied. Adding structured status conditions enables standard Kubernetes observability patterns and unlocks game-server-aware health reporting.

## What Changes

- Define and populate standard Kubernetes condition types (`Ready`, `Progressing`, `Degraded`) on all three CRDs (SatisfactoryServer, SaveGame, ClusterPolicy).
- Define and populate game-server-specific condition types on SatisfactoryServer: `ServerReachable`, `ServerClaimed`, `WorldLoaded`.
- Integrate the existing `pkg/satisfactory` API client into the SatisfactoryServer reconciler to query live game state and reflect it in conditions.
- Populate the currently-empty status fields (`ServerVersion`, `ConnectedPlayers`, `LastSaveTime`) using data from `QueryServerState` and `EnumerateSessions`.
- Add a condition helper package to standardise condition setting/clearing across all controllers.

## Capabilities

### New Capabilities
- `status-conditions`: Standard and game-specific condition types, condition helper utilities, and reconciler integration for all three CRDs.

### Modified Capabilities
- `server-lifecycle`: The SatisfactoryServer reconciler gains game-server health probing and richer status updates during reconciliation.

## Impact

- **API types** (`api/v1alpha1/`): New condition type constants; no schema-breaking changes (conditions array already exists).
- **Controllers** (`internal/controller/`): SatisfactoryServer reconciler adds game API calls and condition updates. SaveGame and ClusterPolicy reconcilers gain basic condition management.
- **New package**: `internal/conditions/` (or similar) for shared condition helpers.
- **Dependencies**: No new external dependencies — uses existing `pkg/satisfactory` client and `k8s.io/apimachinery` condition utilities.
- **RBAC**: No changes needed — status subresource permissions already granted.
