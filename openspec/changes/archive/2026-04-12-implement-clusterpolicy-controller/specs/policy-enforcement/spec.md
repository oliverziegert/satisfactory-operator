## ADDED Requirements

### Requirement: Apply default resource limits
The ClusterPolicy controller SHALL apply `defaultResourceLimits` to SatisfactoryServer resources that have no explicit resource requirements set.

#### Scenario: Server without resource requirements
- **WHEN** a SatisfactoryServer has nil `resources.requests` and nil `resources.limits` and the ClusterPolicy specifies `defaultResourceLimits`
- **THEN** the controller SHALL patch the SatisfactoryServer spec to include the default resource requirements

#### Scenario: Server with explicit resource requirements
- **WHEN** a SatisfactoryServer has non-nil `resources.requests` or `resources.limits`
- **THEN** the controller SHALL NOT modify the server's resource requirements

### Requirement: Apply default storage class
The ClusterPolicy controller SHALL apply `defaultStorageClassName` to SatisfactoryServer resources that have no explicit storage class set.

#### Scenario: Server without storage class
- **WHEN** a SatisfactoryServer has nil `storage.storageClassName` and the ClusterPolicy specifies `defaultStorageClassName`
- **THEN** the controller SHALL patch the SatisfactoryServer spec to include the default storage class name

#### Scenario: Server with explicit storage class
- **WHEN** a SatisfactoryServer has a non-nil `storage.storageClassName`
- **THEN** the controller SHALL NOT modify the server's storage class

### Requirement: Apply default server config
The ClusterPolicy controller SHALL apply `defaultServerConfig` fields (maxPlayers, maxTickRate, skipUpdate) to SatisfactoryServer resources where those fields are at their zero value.

#### Scenario: Server with default maxPlayers
- **WHEN** a SatisfactoryServer has `maxPlayers: 0` (zero value) and the ClusterPolicy specifies `defaultServerConfig.maxPlayers: 8`
- **THEN** the controller SHALL patch the SatisfactoryServer to set `maxPlayers: 8`

#### Scenario: Server with explicit maxPlayers
- **WHEN** a SatisfactoryServer has `maxPlayers: 16`
- **THEN** the controller SHALL NOT modify the server's maxPlayers

### Requirement: Validate allowed image versions
The ClusterPolicy controller SHALL validate SatisfactoryServer image references against `allowedImageVersions`. Non-compliant servers SHALL be reported but not modified.

#### Scenario: Image in allowlist
- **WHEN** a SatisfactoryServer uses `image: wolveix/satisfactory-server:v1.9.10` and `allowedImageVersions` contains `wolveix/satisfactory-server:v1.9.10`
- **THEN** the server SHALL be considered compliant

#### Scenario: Image not in allowlist
- **WHEN** a SatisfactoryServer uses an image not in `allowedImageVersions`
- **THEN** the controller SHALL emit a Warning Event on the SatisfactoryServer and include it in the `Enforced=False` condition message on the ClusterPolicy

#### Scenario: Empty allowlist
- **WHEN** `allowedImageVersions` is empty or nil
- **THEN** all images SHALL be considered compliant (no restriction)

### Requirement: Enforce max servers per namespace
The ClusterPolicy controller SHALL count SatisfactoryServer resources per namespace and report non-compliance when `maxServersPerNamespace` is exceeded.

#### Scenario: Namespace within limit
- **WHEN** a namespace has 3 SatisfactoryServer resources and `maxServersPerNamespace` is 5
- **THEN** the namespace SHALL be considered compliant

#### Scenario: Namespace exceeds limit
- **WHEN** a namespace has 6 SatisfactoryServer resources and `maxServersPerNamespace` is 5
- **THEN** the controller SHALL emit Warning Events and include the namespace in the `Enforced=False` condition message

#### Scenario: No limit set
- **WHEN** `maxServersPerNamespace` is 0 or nil
- **THEN** no per-namespace limit SHALL be enforced

### Requirement: Single active ClusterPolicy
When multiple ClusterPolicy resources exist, the controller SHALL use the oldest one (by creation timestamp) and mark the others as inactive.

#### Scenario: Multiple policies
- **WHEN** two ClusterPolicy resources exist
- **THEN** the controller SHALL apply only the one with the earliest `creationTimestamp` and set `Ready=False` with reason `MultiplePoliciesDetected` on the others