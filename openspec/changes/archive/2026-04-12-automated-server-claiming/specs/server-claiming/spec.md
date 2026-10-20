## ADDED Requirements

### Requirement: Automatic server claiming
When the game server is reachable, unclaimed, and `adminPasswordSecretRef` is configured and `autoClaimDisabled` is not `true`, the controller SHALL automatically claim the server.

#### Scenario: Unclaimed server with credentials configured
- **WHEN** a `SatisfactoryServer` has `ServerReachable=True` and `ServerClaimed=False` and `adminPasswordSecretRef` is set and `autoClaimDisabled` is not `true`
- **THEN** the controller SHALL call `PasswordlessLogin` with `PrivilegeLevelInitialAdmin`, then call `ClaimServer` with the configured `serverName` and the admin password from the Secret

#### Scenario: Successful claim
- **WHEN** `ClaimServer` succeeds and returns an authentication token
- **THEN** the controller SHALL store the token in the `<server-name>-auth-token` Secret, emit a Kubernetes Event with reason `ServerClaimed`, and requeue immediately

#### Scenario: Claim fails because server is already claimed
- **WHEN** `ClaimServer` returns an `APIError` indicating the server is already claimed
- **THEN** the controller SHALL log the error, skip claiming, and proceed with normal reconciliation

#### Scenario: Auto-claim disabled
- **WHEN** a `SatisfactoryServer` has `autoClaimDisabled: true`
- **THEN** the controller SHALL NOT attempt to claim the server, regardless of its claimed state

#### Scenario: No admin password configured
- **WHEN** a `SatisfactoryServer` does not have `adminPasswordSecretRef` set
- **THEN** the controller SHALL NOT attempt to claim the server

### Requirement: serverName spec field
The `SatisfactoryServerSpec` SHALL include an optional `serverName` field of type `string` that specifies the in-game display name set during claiming.

#### Scenario: serverName specified
- **WHEN** a `SatisfactoryServer` resource specifies `serverName: "My Factory"`
- **THEN** the controller SHALL use `"My Factory"` as the `serverName` parameter in `ClaimServer`

#### Scenario: serverName not specified
- **WHEN** a `SatisfactoryServer` resource does not specify `serverName`
- **THEN** the controller SHALL use the Kubernetes resource name (`server.Name`) as the `serverName` parameter in `ClaimServer`

### Requirement: autoClaimDisabled spec field
The `SatisfactoryServerSpec` SHALL include an optional `autoClaimDisabled` boolean field that disables automatic server claiming. It SHALL default to `false`.

#### Scenario: Default auto-claim behavior
- **WHEN** a `SatisfactoryServer` resource is created without `autoClaimDisabled`
- **THEN** automatic claiming SHALL be enabled (the controller will attempt to claim unclaimed servers)

#### Scenario: Auto-claim explicitly disabled
- **WHEN** a `SatisfactoryServer` resource specifies `autoClaimDisabled: true`
- **THEN** the controller SHALL NOT attempt to claim the server
