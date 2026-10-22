## MODIFIED Requirements

### Requirement: Headless Service creation
The operator SHALL create a headless Service (`clusterIP: None`) named `<server-name>-headless` to satisfy the StatefulSet's governing Service requirement and enable stable DNS for the pod.

#### Scenario: Headless Service created alongside StatefulSet
- **WHEN** a `SatisfactoryServer` resource is created
- **THEN** the operator SHALL create a headless Service with `clusterIP: None` exposing game-tcp (TCP), game-udp (UDP), and messaging (TCP) ports

#### Scenario: Headless Service follows port changes
- **WHEN** `gamePort` or `messagingPort` of an existing `SatisfactoryServer` is changed
- **THEN** the operator SHALL update the headless Service's ports to the new values on the next reconciliation

### Requirement: External Service creation
The operator SHALL create an external Service named `<server-name>` using the `serviceType` from the spec (default `LoadBalancer`) to make the server reachable by players.

#### Scenario: LoadBalancer Service created by default
- **WHEN** a `SatisfactoryServer` resource is created without specifying `serviceType`
- **THEN** the operator SHALL create a Service of type `LoadBalancer` exposing game-tcp, game-udp, and messaging ports

#### Scenario: Custom Service type respected
- **WHEN** a `SatisfactoryServer` resource specifies `serviceType: NodePort`
- **THEN** the operator SHALL create a Service of type `NodePort`

#### Scenario: External Service follows port and type changes
- **WHEN** `gamePort`, `messagingPort` or `serviceType` of an existing `SatisfactoryServer` is changed
- **THEN** the operator SHALL update the external Service's type and ports to the new values on the next reconciliation

## ADDED Requirements

### Requirement: NodePort Service pins node ports to the game ports
With `serviceType: NodePort`, the external Service SHALL set each port's `nodePort` equal to its `port`, because game clients connect to the ports the server advertises. With any other Service type, the operator SHALL leave `nodePort` unset.

#### Scenario: NodePort Service uses identical ports
- **WHEN** a `SatisfactoryServer` specifies `serviceType: NodePort`, `gamePort: 30777` and `messagingPort: 30888`
- **THEN** the external Service SHALL expose game-tcp and game-udp with `port` and `nodePort` 30777, and messaging with `port` and `nodePort` 30888

#### Scenario: LoadBalancer Service leaves node ports to the cluster
- **WHEN** a `SatisfactoryServer` uses `serviceType: LoadBalancer`
- **THEN** the operator SHALL NOT set `nodePort` on the external Service's ports

#### Scenario: Switching from LoadBalancer to NodePort
- **WHEN** an existing `SatisfactoryServer` changes `serviceType` from `LoadBalancer` to `NodePort` with ports inside the node-port range
- **THEN** the external Service's node ports SHALL be replaced by the game ports on the next reconciliation

### Requirement: Operator-owned Service ports are authoritative
The operator SHALL overwrite manual changes to the type or ports of the headless and external Services with the desired values, and SHALL update a Service only when its type or ports differ from the desired values.

#### Scenario: Manual node port patch reverted
- **WHEN** a user patches a `nodePort` of the external Service of a `NodePort` server to a different value
- **THEN** the operator SHALL restore `nodePort` to the game port on the next reconciliation

#### Scenario: Unchanged Service not updated
- **WHEN** the type and ports of an existing Service already match the desired values
- **THEN** the operator SHALL NOT send an update for that Service
