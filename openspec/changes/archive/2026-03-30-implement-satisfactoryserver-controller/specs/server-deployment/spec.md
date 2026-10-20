## ADDED Requirements

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

### Requirement: External Service creation
The operator SHALL create an external Service named `<server-name>` using the `serviceType` from the spec (default `LoadBalancer`) to make the server reachable by players.

#### Scenario: LoadBalancer Service created by default
- **WHEN** a `SatisfactoryServer` resource is created without specifying `serviceType`
- **THEN** the operator SHALL create a Service of type `LoadBalancer` exposing game-tcp, game-udp, and messaging ports

#### Scenario: Custom Service type respected
- **WHEN** a `SatisfactoryServer` resource specifies `serviceType: NodePort`
- **THEN** the operator SHALL create a Service of type `NodePort`

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