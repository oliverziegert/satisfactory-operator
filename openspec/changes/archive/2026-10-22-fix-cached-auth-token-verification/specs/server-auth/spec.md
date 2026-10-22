## MODIFIED Requirements

### Requirement: Auth token caching in managed Secret
The controller SHALL cache the authentication token in a Secret named `<server-name>-auth-token` in the same namespace as the `SatisfactoryServer` resource. The token SHALL be stored under key `token`.

#### Scenario: Token cached after successful login
- **WHEN** `PasswordLogin` succeeds and returns an authentication token
- **THEN** the controller SHALL create or update a Secret named `<server-name>-auth-token` with the token value under key `token`

#### Scenario: Managed Secret has owner reference
- **WHEN** the controller creates the `<server-name>-auth-token` Secret
- **THEN** the Secret SHALL have an owner reference pointing to the `SatisfactoryServer` resource so it is garbage collected on deletion

#### Scenario: Cached token reused on subsequent reconciliation
- **WHEN** the `<server-name>-auth-token` Secret exists with a token value
- **THEN** the controller SHALL read the cached token and verify it with an authenticated `QueryServerState` call before using it

### Requirement: Token verification and re-authentication
Before using a cached token, the controller SHALL verify it with an authenticated `QueryServerState` call. If that call returns any error, the controller SHALL re-authenticate via `PasswordLogin` and update the cached token. The log entry for the re-authentication SHALL include the verification error.

#### Scenario: Cached token is valid
- **WHEN** `QueryServerState` with the cached token succeeds
- **THEN** the controller SHALL use the cached token for the current probe, set `Authenticated=True` with reason `TokenValid`, and SHALL NOT call `PasswordLogin`

#### Scenario: Cached token is expired or invalid
- **WHEN** `QueryServerState` with the cached token returns an error
- **THEN** the controller SHALL call `PasswordLogin` to obtain a fresh token, update the `<server-name>-auth-token` Secret, and use the new token

#### Scenario: Server rejects VerifyAuthenticationToken
- **WHEN** the game server rejects `VerifyAuthenticationToken` (for example HTTP 200 with `errorCode` `missing_params`) but accepts the cached token for `QueryServerState`
- **THEN** the controller SHALL treat the cached token as valid and SHALL NOT call `PasswordLogin`
