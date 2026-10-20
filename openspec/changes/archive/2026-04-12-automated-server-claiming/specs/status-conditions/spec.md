## MODIFIED Requirements

### Requirement: ServerClaimed condition
The `ServerClaimed` condition SHALL reflect whether the Satisfactory server has been claimed by an administrator. The controller MAY transition this condition to `True` by automatically claiming the server.

#### Scenario: Server is claimed
- **WHEN** `QueryServerState` succeeds and the server state indicates it has been claimed
- **THEN** `ServerClaimed` SHALL be `True` with reason `ServerClaimed`

#### Scenario: Server is unclaimed
- **WHEN** the server API returns an auth error indicating the server is in initial setup state
- **THEN** `ServerClaimed` SHALL be `False` with reason `ServerUnclaimed`

#### Scenario: Server claimed by controller
- **WHEN** the controller successfully calls `ClaimServer` on an unclaimed server
- **THEN** `ServerClaimed` SHALL transition to `True` with reason `AutoClaimed` on the next reconciliation
