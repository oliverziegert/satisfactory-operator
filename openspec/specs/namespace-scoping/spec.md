# namespace-scoping Specification

## Purpose
Controls which namespaces the operator watches and reconciles, so it can run with permissions limited to the namespaces that host game servers.

## Requirements
### Requirement: Watch-namespaces configuration
The operator SHALL accept a comma-separated list of namespaces to watch via the `--watch-namespaces` flag. When the flag is not set, it SHALL read the `WATCH_NAMESPACES` environment variable. Surrounding whitespace and empty entries SHALL be ignored.

#### Scenario: Flag set
- **WHEN** the manager starts with `--watch-namespaces=games,games-test`
- **THEN** it watches exactly the namespaces `games` and `games-test`

#### Scenario: Env fallback
- **WHEN** the flag is not set and `WATCH_NAMESPACES=" games , "` is set
- **THEN** it watches exactly the namespace `games`

#### Scenario: Flag wins over env
- **WHEN** both `--watch-namespaces=games` and `WATCH_NAMESPACES=other` are set
- **THEN** it watches exactly the namespace `games`

### Requirement: Cluster-wide default
When no watch namespaces are configured, the operator SHALL watch and reconcile resources in all namespaces, as before this change.

#### Scenario: Nothing configured
- **WHEN** neither the flag nor the env var is set, or the value contains only separators and whitespace
- **THEN** the operator reconciles a `SatisfactoryServer` in any namespace

### Requirement: Namespaced resources limited to watched namespaces
When watch namespaces are configured, the operator SHALL reconcile `SatisfactoryServer` and `SaveGame` resources, and read or write their dependent objects, only in the watched namespaces. Resources in other namespaces SHALL be left untouched, and the operator SHALL NOT need permissions outside the watched namespaces for them.

#### Scenario: Server in a watched namespace
- **WHEN** watch namespaces are `games` and a `SatisfactoryServer` is created in `games`
- **THEN** the operator creates its StatefulSet and Services

#### Scenario: Server in an unwatched namespace
- **WHEN** watch namespaces are `games` and a `SatisfactoryServer` is created in `other`
- **THEN** the operator creates nothing for it and does not update its status

### Requirement: ClusterPolicy in namespace mode
`ClusterPolicy` SHALL be watched cluster-wide in both modes. When watch namespaces are configured, the active policy's defaults and constraints SHALL apply only to `SatisfactoryServer` resources in watched namespaces.

#### Scenario: Defaults applied in a watched namespace
- **WHEN** watch namespaces are `games` and a `ClusterPolicy` with `defaultServerConfig.maxPlayers: 8` exists
- **THEN** a server in `games` without `maxPlayers` is patched to `8`

#### Scenario: Unwatched server not counted
- **WHEN** watch namespaces are `games` and `maxServersPerNamespace: 1` is set and namespace `other` holds two servers
- **THEN** the policy's `Enforced` condition does not report `other` as a violation
