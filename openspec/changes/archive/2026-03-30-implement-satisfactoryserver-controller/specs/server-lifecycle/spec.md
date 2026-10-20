## MODIFIED Requirements

### Requirement: Controller reconciliation stub
The SatisfactoryServer controller SHALL implement the `Reconcile` method with a full reconciliation loop that creates and manages a StatefulSet, headless Service, and external Service, adds a finalizer, handles deletion, and updates the resource's Phase status.

#### Scenario: Controller registration
- **WHEN** the operator manager starts
- **THEN** the SatisfactoryServer controller SHALL be registered with the manager and begin watching `SatisfactoryServer` resources

#### Scenario: Full reconciliation on create
- **WHEN** a `SatisfactoryServer` resource is created
- **THEN** the controller SHALL add a finalizer, create a StatefulSet, a headless Service, and an external Service in the same namespace

#### Scenario: Phase set to Pending while server starts
- **WHEN** a `SatisfactoryServer` resource is reconciled and the StatefulSet has zero ready replicas
- **THEN** the controller SHALL set `status.phase` to `Pending`

#### Scenario: Phase set to Running when server is ready
- **WHEN** the StatefulSet managed by a `SatisfactoryServer` has at least one ready replica
- **THEN** the controller SHALL set `status.phase` to `Running`

## ADDED Requirements

### Requirement: RBAC for StatefulSets and Services
The SatisfactoryServer controller SHALL have RBAC permissions to create, get, list, watch, update, patch, and delete `StatefulSets` and `Services` in addition to the existing Pod, PVC, ConfigMap, and SatisfactoryServer permissions.

#### Scenario: RBAC markers for new resources
- **WHEN** the controller source code is processed by controller-gen
- **THEN** the generated ClusterRole SHALL include rules for `apps/statefulsets` and `core/services`