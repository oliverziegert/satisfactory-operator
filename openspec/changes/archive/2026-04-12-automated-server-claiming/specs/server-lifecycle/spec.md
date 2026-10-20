## MODIFIED Requirements

### Requirement: SatisfactoryServer spec fields
- **WHEN** a `SatisfactoryServer` custom resource is defined
- **THEN** the spec SHALL include fields for: server configuration (server name, admin password, max players, auto-pause, auto-save interval), resource requirements (CPU, memory requests/limits), persistent storage (size, storage class), networking (game port, query port, API port), image version, `adminPasswordSecretRef`, and the new fields `serverName` (string, optional) and `autoClaimDisabled` (bool, optional, default false)

#### Scenario: SatisfactoryServer spec fields
- **WHEN** a `SatisfactoryServer` custom resource is defined
- **THEN** the spec SHALL include all existing fields plus `serverName` (in-game display name) and `autoClaimDisabled` (opt-out of automatic claiming)

### Requirement: Controller reconciliation stub
The SatisfactoryServer controller SHALL implement the `Reconcile` method with a full reconciliation loop that creates and manages a StatefulSet, headless Service, and external Service, adds a finalizer, handles deletion, authenticates with the game server API when credentials are configured, claims unclaimed servers when auto-claiming is enabled, updates status conditions by probing the game server API, and updates the resource's Phase status.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the SatisfactoryServer controller SHALL be registered with the manager and begin watching `SatisfactoryServer` resources

#### Scenario: Full reconciliation on create
- **WHEN** a `SatisfactoryServer` resource is created
- **THEN** the controller SHALL add a finalizer, create a StatefulSet, a headless Service, and an external Service in the same namespace, and set initial conditions (`Ready=False`, `Progressing=True`, `ServerReachable=False`)

#### Scenario: Claiming step in reconciliation
- **WHEN** the probe determines `ServerClaimed=False` and `ServerReachable=True` and auto-claiming is enabled
- **THEN** the controller SHALL attempt to claim the server before completing the reconciliation loop

#### Scenario: Game server API probing during reconciliation
- **WHEN** the StatefulSet has ready replicas
- **THEN** the controller SHALL authenticate if configured, probe the server, attempt claiming if unclaimed and enabled, then update all conditions accordingly
