# Proposal

## Why

A running SatisfactoryServer is never re-probed, so `status.connectedPlayers` and the game conditions go stale until an unrelated watch event triggers a reconcile (#40). In the second e2e test, one player was connected while the `PLAYERS` column stayed blank. The column is also blank for 0 players, and `status.serverVersion` is never populated.

## What Changes

- A server whose reconcile reaches the end is requeued every 60 seconds, so the probe refreshes `connectedPlayers`, `ServerReachable`, `ServerClaimed`, `WorldLoaded`, `Authenticated` and `Degraded`. The 5-second (API unreachable) and 30-second (pod not ready) requeues stay as they are.
- `status.connectedPlayers` is set only when the game API returns the server state. A reachable server with no players shows `0`. When the count is unknown (pod not ready, API unreachable or not authenticated), the field is unset and the column is blank. The Go type becomes `*int32`.
- **BREAKING** (v1alpha1, field was always empty): `status.serverVersion` is removed. The HTTPS game API exposes no server version.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `status-conditions`: periodic re-probe of a reachable server (replaces "no explicit requeue"), `connectedPlayers` is unset when unknown, and the `serverVersion` requirement is removed.
- `server-lifecycle`: the "Status fields populated from game API" scenario no longer mentions `status.serverVersion`.

## Impact

- `api/v1alpha1/satisfactoryserver_types.go`: `ConnectedPlayers` becomes `*int32`, `ServerVersion` is removed. The CRD (`config/crd/bases/`) and deepcopy are regenerated.
- `internal/controller/satisfactoryserver_controller.go`: the final requeue, `probeResult`, `probeGameServer` and `setGameConditions`.
- `internal/controller/satisfactoryserver_probe_test.go`: player-count assertions.
- Load: each running server makes up to three HTTPS calls to its game API per minute (token verification, `QueryServerState`, claim check).
- Patch release (`fix:`), fixes #40.
