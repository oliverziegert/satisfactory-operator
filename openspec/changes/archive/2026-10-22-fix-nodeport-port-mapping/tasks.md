# Tasks

## 1. Pin node ports on NodePort Services

- [x] 1.1 In `buildExternalService` (`internal/controller/satisfactoryserver_helpers.go`), set `NodePort = Port` on every port when the resolved type is `NodePort`, and verify with a new `TestBuildExternalService` case in `satisfactoryserver_helpers_test.go` that NodePort with 30777/30888 yields `nodePort == port` for game-tcp, game-udp and messaging, and that LoadBalancer yields `nodePort == 0`
- [x] 1.2 Verify that the headless Service never carries node ports: `buildHeadlessService` with `serviceType: NodePort` has `nodePort == 0` on all ports (unit test)

## 2. Reconcile Service type and ports

- [x] 2.1 Add `servicePortsMatch(existing, desired []corev1.ServicePort) bool` to `satisfactoryserver_helpers.go` (compares `Name`/`Protocol`/`Port`, and `NodePort` only where desired is non-zero), and verify with a table test covering: equal, changed port, allocated node port ignored, pinned node port differs, different length
- [x] 2.2 Make `reconcileExternalService` update `Spec.Type` and `Spec.Ports` when the type differs or `servicePortsMatch` is false, and skip `Update` otherwise. Verify with an envtest case: changing `gamePort` on an existing server updates the external Service's ports, and the allocated node ports of the `LoadBalancer` Service stay unchanged for unchanged ports
- [x] 2.3 Make `reconcileHeadlessService` update `Spec.Ports` when `servicePortsMatch` is false. Verify in the same envtest case that the headless Service follows the `gamePort` change
- [x] 2.4 Add an envtest case for the switch from `LoadBalancer` to `NodePort` with 30777/30888, and verify that the external Service ends up with `type: NodePort` and `nodePort == port` on all three ports

## 3. Report a rejected external Service

- [x] 3.1 In `Reconcile` (`satisfactoryserver_controller.go`), keep an `apierrors.IsInvalid` error from `reconcileExternalService` instead of returning it, and keep returning all other errors immediately
- [x] 3.2 After step 10a, when the Service error is set: set `Degraded=True` and `Ready=False` with reason `ServiceInvalid` and the API server message, emit a Warning Event `ServiceInvalid` through `r.Recorder`, write status, then return the error. Verify with an envtest case: `serviceType: NodePort` with the default `gamePort` 7777 yields `Degraded=True`/`ServiceInvalid` with a message mentioning the valid range, `Ready=False` and `status.phase == Error`
- [x] 3.3 Extend the same envtest case for recovery: change `gamePort`/`messagingPort` to 30777/30888, and verify that `Degraded` no longer has reason `ServiceInvalid` and the external Service exists with the pinned node ports
- [x] 3.4 Update the `Degraded` doc comment on `SatisfactoryServerConditionTypeDegraded` in `api/v1alpha1/satisfactoryserver_types.go` to mention the rejected Service. Run `make manifests generate` and verify there is no unexpected diff beyond the comment-derived CRD description

## 4. Documentation

- [x] 4.1 Add a "NodePort" note to `README.md`: both ports must be in the node-port range (default 30000–32767) and are used unchanged as container, Service and node port, the operator reverts manual port patches, and an out-of-range port shows as `Degraded`/`ServiceInvalid`. Verify the README renders and the commands in it are unchanged
- [x] 4.2 Add a commented NodePort example (`serviceType: NodePort`, `gamePort: 30777`, `messagingPort: 30888`) to `config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml`, and verify with `kubectl apply --dry-run=client -f` on the uncommented variant

## 5. Integration checks

- [x] 5.1 Run `make lint` and `make test`, and verify both pass
- [x] 5.2 Run `openspec validate fix-nodeport-port-mapping --strict`, and verify it passes

## Workflow follow-up

- Open a PR titled `fix: pin NodePort ports to the game ports and reconcile Service ports` that references #38.
- Optionally re-test on the single-node Talos cluster: join a world with `serviceType: NodePort`, 30777/30888 and no manual Service patches.
- Archive the change after merge.
