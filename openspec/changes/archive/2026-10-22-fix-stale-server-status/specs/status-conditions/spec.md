# Spec Delta

## MODIFIED Requirements

### Requirement: Populate ConnectedPlayers from game API
The reconciler SHALL set `status.connectedPlayers` from the `numConnectedPlayers` field returned by `QueryServerState`. When the reconciler cannot read the player count, it SHALL leave `status.connectedPlayers` unset, so an unknown count is distinguishable from zero players.

#### Scenario: Players connected
- **WHEN** `QueryServerState` returns `numConnectedPlayers=3`
- **THEN** `status.connectedPlayers` SHALL be `3`

#### Scenario: No players connected
- **WHEN** `QueryServerState` returns `numConnectedPlayers=0`
- **THEN** `status.connectedPlayers` SHALL be `0` and the `Players` print column SHALL show `0`

#### Scenario: Server unreachable
- **WHEN** the game server API is unreachable
- **THEN** `status.connectedPlayers` SHALL be unset

#### Scenario: Game state not readable
- **WHEN** the game server API rejects `QueryServerState` with an API error (for example, the operator is not authenticated)
- **THEN** `status.connectedPlayers` SHALL be unset

#### Scenario: Pod not ready
- **WHEN** the StatefulSet has no ready replicas and the game server API is not probed
- **THEN** `status.connectedPlayers` SHALL be unset, even if an earlier probe had set it

### Requirement: Short requeue when server unreachable
The reconciler SHALL requeue with a 5-second delay when `ServerReachable` is `False` to quickly detect when the game server becomes available. Otherwise it SHALL requeue after 60 seconds, so a running server is re-probed and its player count and game conditions stay current without a watch event.

#### Scenario: Requeue on unreachable
- **WHEN** the game server API probe fails
- **THEN** the reconciler SHALL return `ctrl.Result{RequeueAfter: 5 * time.Second}`

#### Scenario: Normal requeue when reachable
- **WHEN** the game server API probe succeeds
- **THEN** the reconciler SHALL return `ctrl.Result{RequeueAfter: 60 * time.Second}`

#### Scenario: Player joins a running server
- **WHEN** a player joins a server that is `Running` and no spec, StatefulSet, Service or Secret change occurs
- **THEN** `status.connectedPlayers` SHALL reflect the new player count within about 60 seconds

## REMOVED Requirements

### Requirement: Populate ServerVersion from game API
**Reason**: The Satisfactory Dedicated Server HTTPS API (spec v0.2.1) returns no server version, so `status.serverVersion` was never populated.
**Migration**: None. The field was always empty. Read the game version from the container image tag or the server log.
