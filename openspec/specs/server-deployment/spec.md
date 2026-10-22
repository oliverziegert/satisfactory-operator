# server-deployment Specification

## Purpose
Create the StatefulSet, headless Service and external Service for a SatisfactoryServer, and control PVC cleanup on deletion.

## Requirements
### Requirement: StatefulSet creation
The operator SHALL create a `StatefulSet` with 1 replica for each `SatisfactoryServer` resource, using the Pod spec built from the CRD spec fields and a `volumeClaimTemplate` named `config` for the `/config` persistent volume.

#### Scenario: StatefulSet created on new resource
- **WHEN** a `SatisfactoryServer` resource is created
- **THEN** the operator SHALL create a `StatefulSet` in the same namespace with `replicas: 1`, a pod template matching the server spec, and a `volumeClaimTemplate` using the configured storage size and storage class

#### Scenario: StatefulSet reflects spec changes
- **WHEN** a `SatisfactoryServer` spec field is updated (e.g. image, maxPlayers)
- **THEN** the operator SHALL update the existing `StatefulSet` to reflect the new spec on the next reconciliation

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

### Requirement: Finalizer-based deletion protection
The operator SHALL add a finalizer `satisfactory.oliverziegert.github.com/finalizer` to every `SatisfactoryServer` resource to control the deletion sequence.

#### Scenario: Finalizer added on creation
- **WHEN** a `SatisfactoryServer` resource is created without the finalizer
- **THEN** the operator SHALL add the finalizer before creating any child resources

#### Scenario: Deletion with PVC retention (default)
- **WHEN** a `SatisfactoryServer` resource is deleted and `retainPVCOnDelete` is `true` (or unset)
- **THEN** the operator SHALL remove the finalizer without deleting the PVC, leaving save data intact

#### Scenario: Deletion with PVC cleanup
- **WHEN** a `SatisfactoryServer` resource is deleted and `retainPVCOnDelete` is `false`
- **THEN** the operator SHALL delete the PVC named `config-<server-name>-0` before removing the finalizer

### Requirement: ServiceType field on SatisfactoryServer spec
The `SatisfactoryServer` spec SHALL include a `serviceType` field of type `corev1.ServiceType` that controls the Kubernetes Service type used to expose the server. It SHALL default to `LoadBalancer`.

#### Scenario: Default service type
- **WHEN** a `SatisfactoryServer` resource is created without a `serviceType` field
- **THEN** the external Service SHALL be of type `LoadBalancer`

### Requirement: RetainPVCOnDelete field on SatisfactoryServer spec
The `SatisfactoryServer` spec SHALL include a `retainPVCOnDelete` boolean field that controls whether the `/config` PVC is preserved when the resource is deleted. It SHALL default to `true`.

#### Scenario: Default PVC retention
- **WHEN** a `SatisfactoryServer` resource is deleted without specifying `retainPVCOnDelete`
- **THEN** the PVC SHALL be retained after deletion

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
