## MODIFIED Requirements

### Requirement: SatisfactoryServer spec fields
- **WHEN** a `SatisfactoryServer` custom resource is defined
- **THEN** the spec SHALL include fields for: server configuration (server name, admin password, max players, auto-pause, auto-save interval), resource requirements (CPU, memory requests/limits), persistent storage (size, storage class), networking (game port, query port, API port), image version, and an optional `adminPasswordSecretRef` of type `corev1.LocalObjectReference` referencing a Secret containing the admin password

#### Scenario: SatisfactoryServer spec fields
- **WHEN** a `SatisfactoryServer` custom resource is defined
- **THEN** the spec SHALL include all existing fields plus `adminPasswordSecretRef` which is optional and references a Secret by name in the same namespace

### Requirement: Controller reconciliation stub
The SatisfactoryServer controller SHALL implement the `Reconcile` method with a full reconciliation loop that creates and manages a StatefulSet, headless Service, and external Service, adds a finalizer, handles deletion, authenticates with the game server API when credentials are configured, updates status conditions by probing the game server API, and updates the resource's Phase status.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the SatisfactoryServer controller SHALL be registered with the manager and begin watching `SatisfactoryServer` resources

#### Scenario: Full reconciliation on create
- **WHEN** a `SatisfactoryServer` resource is created
- **THEN** the controller SHALL add a finalizer, create a StatefulSet, a headless Service, and an external Service in the same namespace, and set initial conditions (`Ready=False`, `Progressing=True`, `ServerReachable=False`, `Authenticated=Unknown`)

#### Scenario: Phase set to Pending while server starts
- **WHEN** a `SatisfactoryServer` resource is reconciled and the StatefulSet has zero ready replicas
- **THEN** the controller SHALL set `status.phase` to `Pending` and `Progressing=True`

#### Scenario: Phase set to Running when server is ready
- **WHEN** the StatefulSet managed by a `SatisfactoryServer` has at least one ready replica AND the game server API is reachable
- **THEN** the controller SHALL set `status.phase` to `Running` and update all game-specific conditions

#### Scenario: Game server API probing during reconciliation
- **WHEN** the StatefulSet has ready replicas
- **THEN** the controller SHALL authenticate if `adminPasswordSecretRef` is configured, then call `QueryServerState()` via the game API client and update `ServerReachable`, `ServerClaimed`, `WorldLoaded`, `Degraded`, `Authenticated`, and `Ready` conditions accordingly

#### Scenario: Status fields populated from game API
- **WHEN** `QueryServerState` returns successfully
- **THEN** the controller SHALL populate `status.connectedPlayers` and `status.serverVersion` from the response
