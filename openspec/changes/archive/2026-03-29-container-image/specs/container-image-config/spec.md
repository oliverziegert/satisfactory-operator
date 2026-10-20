## ADDED Requirements

### Requirement: Default container image
The operator SHALL use `wolveix/satisfactory-server:v1.9.10` as the default container image for SatisfactoryServer resources when no image is specified in the CRD spec.

#### Scenario: No image specified
- **WHEN** a SatisfactoryServer resource is created without an `image` field
- **THEN** the operator SHALL create a Pod using `wolveix/satisfactory-server:v1.9.10`

#### Scenario: Custom image specified
- **WHEN** a SatisfactoryServer resource specifies `image: wolveix/satisfactory-server:v1.8.7`
- **THEN** the operator SHALL create a Pod using the specified image and tag

### Requirement: Container port definitions
The operator SHALL configure the following container ports on the game server Pod: 7777/TCP (server query and HTTPS API), 7777/UDP (game traffic), and 8888/TCP (reliable messaging).

#### Scenario: Default port configuration
- **WHEN** a SatisfactoryServer resource is created with default settings
- **THEN** the Pod SHALL expose ports 7777/TCP, 7777/UDP, and 8888/TCP

#### Scenario: Custom game port
- **WHEN** a SatisfactoryServer resource specifies `gamePort: 15777`
- **THEN** the Pod SHALL expose port 15777/TCP, 15777/UDP, and the container SHALL have `SERVERGAMEPORT=15777` environment variable

#### Scenario: Custom messaging port
- **WHEN** a SatisfactoryServer resource specifies `messagingPort: 18888`
- **THEN** the Pod SHALL expose port 18888/TCP and the container SHALL have `SERVERMESSAGINGPORT=18888` environment variable

### Requirement: Persistent volume mount
The operator SHALL create a PersistentVolumeClaim for each SatisfactoryServer and mount it at `/config` in the game server container.

#### Scenario: Default storage
- **WHEN** a SatisfactoryServer resource is created without storage configuration
- **THEN** the operator SHALL create a PVC with 15Gi storage request and mount it at `/config`

#### Scenario: Custom storage size and class
- **WHEN** a SatisfactoryServer resource specifies `storage.size: 50Gi` and `storage.storageClassName: fast-ssd`
- **THEN** the PVC SHALL request 50Gi with storageClassName `fast-ssd`

### Requirement: Environment variable mapping for server configuration
The operator SHALL map CRD spec fields to the container's environment variables as follows: `maxPlayers` to `MAXPLAYERS`, `autosaveNum` to `AUTOSAVENUM`, `maxTickRate` to `MAXTICKRATE`, `maxObjects` to `MAXOBJECTS`, `serverStreaming` to `SERVERSTREAMING`, `disableSeasonalEvents` to `DISABLESEASONALEVENTS`, and `timeout` to `TIMEOUT`.

#### Scenario: Max players configured
- **WHEN** a SatisfactoryServer resource specifies `maxPlayers: 8`
- **THEN** the container SHALL have environment variable `MAXPLAYERS=8`

#### Scenario: Default values applied
- **WHEN** a SatisfactoryServer resource is created without specifying `maxPlayers`
- **THEN** the container SHALL have environment variable `MAXPLAYERS=4` (image default)

### Requirement: Environment variable mapping for update control
The operator SHALL map CRD spec fields to the container's update-related environment variables: `skipUpdate` to `SKIPUPDATE`, `steamBeta` to `STEAMBETA`, and `steamBetaPassword` to `STEAMBETAKEY`.

#### Scenario: Skip update enabled by default
- **WHEN** a SatisfactoryServer resource is created without specifying `skipUpdate`
- **THEN** the container SHALL have environment variable `SKIPUPDATE=true`

#### Scenario: Beta branch enabled
- **WHEN** a SatisfactoryServer resource specifies `steamBeta: true`
- **THEN** the container SHALL have environment variables `STEAMBETA=true`

### Requirement: Environment variable mapping for user identity
The operator SHALL map CRD spec fields to the container's PUID and PGID environment variables for file ownership.

#### Scenario: Custom user and group ID
- **WHEN** a SatisfactoryServer resource specifies `runAsUser: 1001` and `runAsGroup: 1001`
- **THEN** the container SHALL have environment variables `PUID=1001` and `PGID=1001`

#### Scenario: Default user and group ID
- **WHEN** a SatisfactoryServer resource is created without user/group configuration
- **THEN** the container SHALL have environment variables `PUID=1000` and `PGID=1000`

### Requirement: Default resource requirements
The operator SHALL set default resource requirements for the game server container: 4Gi memory request and 8Gi memory limit.

#### Scenario: Default resources
- **WHEN** a SatisfactoryServer resource is created without resource configuration
- **THEN** the Pod SHALL have memory request of 4Gi and memory limit of 8Gi

#### Scenario: Custom resources
- **WHEN** a SatisfactoryServer resource specifies custom resource requests and limits
- **THEN** the Pod SHALL use the specified values instead of defaults

### Requirement: Debug and logging environment variables
The operator SHALL map CRD spec fields to the container's debug and logging environment variables: `debug` to `DEBUG` and `log` to `LOG`.

#### Scenario: Debug mode enabled
- **WHEN** a SatisfactoryServer resource specifies `debug: true`
- **THEN** the container SHALL have environment variable `DEBUG=true`

#### Scenario: Default logging
- **WHEN** a SatisfactoryServer resource is created without debug/log configuration
- **THEN** the container SHALL have environment variables `DEBUG=false` and `LOG=false`

### Requirement: Multihome binding
The operator SHALL map the `multihome` CRD spec field to the `MULTIHOME` environment variable for controlling the server's listening interface.

#### Scenario: Default multihome
- **WHEN** a SatisfactoryServer resource is created without multihome configuration
- **THEN** the container SHALL have environment variable `MULTIHOME=::` (listen on all interfaces)

#### Scenario: Custom multihome
- **WHEN** a SatisfactoryServer resource specifies `multihome: 0.0.0.0`
- **THEN** the container SHALL have environment variable `MULTIHOME=0.0.0.0`