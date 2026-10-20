## Why

The SatisfactoryServer controller is currently a no-op stub — applying a `SatisfactoryServer` resource to a cluster does nothing. This change implements the core reconciliation logic so that a `SatisfactoryServer` CR actually creates and manages a running Satisfactory Dedicated Server on Kubernetes.

## What Changes

- Implement `SatisfactoryServerReconciler.Reconcile()` with full create/update/delete lifecycle
- Add `ServiceType` field to `SatisfactoryServerSpec` — controls how the server is exposed externally (default: `LoadBalancer`)
- Add `RetainPVCOnDelete` field to `SatisfactoryServerSpec` — controls whether save data is kept on resource deletion (default: `true`)
- Add three new helper functions: `buildStatefulSet`, `buildHeadlessService`, `buildExternalService`
- Add finalizer to prevent accidental data loss on deletion
- Update status `Phase` to reflect StatefulSet readiness (Pending → Running)

## Capabilities

### New Capabilities

- `server-deployment`: The controller creates and manages a StatefulSet, headless Service, and external Service for each SatisfactoryServer resource, with finalizer-based deletion protection and basic Phase status tracking.

### Modified Capabilities

- `server-lifecycle`: New CRD spec fields (`serviceType`, `retainPVCOnDelete`) extend the server configuration surface. Reconciliation loop now manages the full lifecycle from creation to deletion.

## Impact

- `api/v1alpha1/satisfactoryserver_types.go` — two new spec fields
- `internal/controller/satisfactoryserver_controller.go` — full Reconcile() implementation replacing the stub
- `internal/controller/satisfactoryserver_helpers.go` — three new builder functions
- `config/crd/bases/` — regenerated CRD manifests
- `config/samples/` — updated sample CR
