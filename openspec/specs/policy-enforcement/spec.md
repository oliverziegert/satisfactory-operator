# policy-enforcement Specification

## Purpose
Apply ClusterPolicy defaults to SatisfactoryServer resources and enforce its constraints on images and servers per namespace.

## Requirements
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
The ClusterPolicy controller SHALL validate SatisfactoryServer image references against `allowedImageVersions`. A reference SHALL be compliant only if it exactly equals an allowlist entry, including registry, repository and tag; different spellings of the same image (e.g. `robtme/…` vs `docker.io/robtme/…`) are distinct. Non-compliant servers SHALL be reported but not modified.

#### Scenario: Image in allowlist
- **WHEN** a SatisfactoryServer uses `image: ghcr.io/robtme/satisfactory-server:v1.9.10` and `allowedImageVersions` contains `ghcr.io/robtme/satisfactory-server:v1.9.10`
- **THEN** the server SHALL be considered compliant

#### Scenario: Same image under a different registry spelling
- **WHEN** a SatisfactoryServer uses `image: docker.io/robtme/satisfactory-server:v1.9.10` and `allowedImageVersions` contains only `robtme/satisfactory-server:v1.9.10`
- **THEN** the server SHALL be considered non-compliant

#### Scenario: Same repository with a different tag
- **WHEN** a SatisfactoryServer uses `image: ghcr.io/robtme/satisfactory-server:latest` and `allowedImageVersions` contains only `ghcr.io/robtme/satisfactory-server:v1.9.10`
- **THEN** the server SHALL be considered non-compliant

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
