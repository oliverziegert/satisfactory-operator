# Proposal

## Why

With `serviceType: NodePort`, players can add the server in the Server Manager but can't join the world (#38). Kubernetes assigns random node ports, while the game advertises the ports it listens on. Joining only works when container port, Service port and node port are identical. The operator also never updates Service ports after creation, so a `gamePort`/`messagingPort` change leaves both Services pointing at the old ports. A rejected Service is only logged, not reported in the resource status.

## What Changes

- With `serviceType: NodePort`, the external Service pins `nodePort` to the Service port for game TCP, game UDP and messaging. Other Service types leave `nodePort` unset.
- The operator reconciles the ports of the headless and external Services on every reconcile. Desired ports overwrite manual changes. A Service is only updated when its type or ports differ.
- When the API server rejects the external Service (for example a node port outside the cluster's node-port range, or a port collision), the operator sets `Degraded=True` with reason `ServiceInvalid` and the API server message, sets `Ready=False`, emits a Warning Event and still writes status. The phase becomes `Error` through the existing phase derivation.
- README and the sample document that with `NodePort`, `gamePort` and `messagingPort` must lie in the node-port range (default 30000–32767).
- Deviations from the issue's proposed fix:
  - No CEL range rule, because the node-port range is an API server flag and isn't readable through the API. The API server validates it instead.
  - Port defaults stay at 7777/8888. `LoadBalancer` remains the default type, and NodePort users must set both ports explicitly.

## Capabilities

### New Capabilities

### Modified Capabilities
- `server-deployment`: The headless and external Service requirements gain port reconciliation, and the external Service requirement gains node-port pinning for `NodePort`.
- `status-conditions`: `Degraded` and `Ready` also reflect a rejected external Service.

## Impact

- Code: `internal/controller/satisfactoryserver_helpers.go` (`buildExternalService`), `internal/controller/satisfactoryserver_controller.go` (`reconcileHeadlessService`, `reconcileExternalService`, condition handling).
- Tests: `internal/controller/satisfactoryserver_helpers_test.go`, envtest suite in `internal/controller/`.
- Docs: `README.md`, `config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml`.
- No CRD/API change, no new dependencies, no RBAC change (the controller can already update Services and create Events).
- Behavior change for existing installations: manual `nodePort`/port patches on operator-owned Services are reverted on the next reconcile.
