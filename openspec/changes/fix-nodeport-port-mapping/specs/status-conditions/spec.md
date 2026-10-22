## MODIFIED Requirements

### Requirement: SatisfactoryServer Ready condition aggregation
The `Ready` condition on SatisfactoryServer SHALL be `True` only when the StatefulSet has ready replicas AND `ServerReachable` is `True` AND `WorldLoaded` is `True` AND the external Service was accepted by the API server.

#### Scenario: All sub-conditions true
- **WHEN** the StatefulSet has ready replicas AND the game server API responds successfully AND an active session is loaded
- **THEN** the `Ready` condition SHALL be `True` with reason `AllChecksPass`

#### Scenario: Server unreachable
- **WHEN** the StatefulSet has ready replicas but the game server API is unreachable
- **THEN** the `Ready` condition SHALL be `False` with reason `ServerUnreachable`

#### Scenario: No active session
- **WHEN** the server is reachable but no active session is loaded
- **THEN** the `Ready` condition SHALL be `False` with reason `NoActiveSession`

#### Scenario: External Service rejected
- **WHEN** the server is reachable and a world is loaded, but the API server rejected the create or update of the external Service
- **THEN** the `Ready` condition SHALL be `False`

## ADDED Requirements

### Requirement: Degraded condition reflects a rejected external Service
When the API server rejects the create or update of the external Service, the operator SHALL set `Degraded=True` with reason `ServiceInvalid` and the API server's message, emit a Warning Event with the same message, and still write the resource status. This reason SHALL take precedence over the `Degraded` value derived from the API probe.

#### Scenario: Node port outside the cluster's range
- **WHEN** a `SatisfactoryServer` specifies `serviceType: NodePort` with `gamePort: 7777`
- **THEN** `Degraded` SHALL be `True` with reason `ServiceInvalid` and a message containing the API server's valid node-port range, `Ready` SHALL be `False`, `status.phase` SHALL be `Error`, and a Warning Event SHALL be recorded on the `SatisfactoryServer`

#### Scenario: Probe result does not mask the Service error
- **WHEN** the external Service is rejected AND the game server API is reachable
- **THEN** `Degraded` SHALL remain `True` with reason `ServiceInvalid`

#### Scenario: Recovery after the spec is fixed
- **WHEN** the user changes the ports so that the API server accepts the external Service
- **THEN** the operator SHALL no longer report reason `ServiceInvalid` and SHALL derive `Degraded` from the API probe again
