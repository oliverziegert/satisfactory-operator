# Design

## Context

See proposal.md for the motivation. Current state in `internal/controller/`:

- `containerPortsToServicePorts` builds `ServicePort`s with `Name`, `Protocol` and `Port` only. `TargetPort` defaults to `Port`, and `NodePort` is left to the API server.
- `reconcileHeadlessService` only creates. `reconcileExternalService` creates and otherwise only updates `Spec.Type`.
- `Reconcile` returns immediately on any error from steps 4–6. Status is not written in that case.
- `setGameConditions` sets `Degraded` from the probe on every reconcile, and `setPodFailureCondition` (step 10a) overrides it for crash loops. `updatePhaseFromConditions` maps `Degraded=True` to `Phase=Error`.
- The node-port range is the kube-apiserver flag `--service-node-port-range` and is not exposed by any API.

## Goals / Non-Goals

**Goals:**
- Players can join a `NodePort` server without patching the Service by hand.
- Port and type changes reach both Services.
- A rejected external Service is visible on the `SatisfactoryServer` status.

**Non-Goals:**
- `hostPort`/`hostNetwork` mode for keeping 7777/8888 on single-node clusters.
- Named `targetPort`s. The game requires identical numbers on every layer, so names add nothing.
- Validating the node-port range in the CRD (CEL) or in the controller.
- Reconciling Service fields other than `Type` and `Ports` (annotations, `externalTrafficPolicy`, `loadBalancerIP`, ...).

## Decisions

**1. Pin `nodePort` in `buildExternalService`, not in `containerPortsToServicePorts`.**
The shared converter also feeds the headless Service, which must never carry node ports. `buildExternalService` sets `NodePort = Port` on each port when the resolved type is `NodePort`.

**2. Compare only the fields the operator owns.**
A new helper `servicePortsMatch(existing, desired)` compares the ports by `Name`, `Protocol` and `Port`, and compares `NodePort` only where the desired value is non-zero. On a mismatch in type or ports, both reconcile functions set `existing.Spec.Type`/`existing.Spec.Ports` to the desired values and call `Update`. Otherwise they do nothing.
- This ignores API-server-defaulted fields (`TargetPort`, allocated `NodePort`), so there are no update loops.
- When `nodePort` is unset in an update of a `LoadBalancer`/`NodePort` Service, the API server keeps the allocation of a matching existing port. Rejected alternative: copying allocated node ports by hand, which duplicates API server behavior.
- Switching `NodePort` → `LoadBalancer` keeps the previously pinned node ports, which is harmless.

**3. Treat only `Invalid` errors from the external Service as a status condition.**
In step 6, `apierrors.IsInvalid(err)` (HTTP 422, which covers an out-of-range or already allocated node port) is kept in a local `svcErr` while reconcile continues. All other errors (conflicts, transient API errors) still return immediately as today. After step 10a, when `svcErr != nil`:
- `Degraded=True`, reason `ServiceInvalid`, message = API server error (it names the valid range)
- `Ready=False`, reason `ServiceInvalid`
- a Warning Event, reason `ServiceInvalid`, through the existing `Recorder`

Setting the condition last makes it win over the probe and pod-failure values. Phase becomes `Error` through the unchanged `updatePhaseFromConditions`. After the status write, `Reconcile` returns `svcErr`, so controller-runtime retries with backoff. This matters because a port collision can clear without a spec change, for example when the other Service is deleted.
- Rejected alternative: map it to `Running` with only `Degraded`. That needs a phase exception and contradicts #37's `Degraded` ⇒ `Error`.

**4. The headless Service gets the same port update but no condition handling.**
Its ports are a subset of what the external Service validates (no node ports), so an `Invalid` there would also fail the external Service. Its errors keep returning immediately.

## Risks / Trade-offs

- [Manual Service port patches, such as the #38 workaround, are reverted] → Intended. The pinned ports make them unnecessary. Called out in the proposal and the README.
- [Warning Event on every retry while the Service is invalid] → The event recorder aggregates repeated events, and controller-runtime's backoff limits the rate.
- [The API server could reallocate a `LoadBalancer` node port on update if the existing-allocation behavior differs from expectations] → Updates only happen on a real port/type change. The envtest port-change case checks that the allocated node ports of a `LoadBalancer` Service are unchanged.
- [`Degraded`/`Ready` reason `ServiceInvalid` hides the probe reason while the Service is invalid] → `ServerReachable` and `WorldLoaded` still carry the probe state.

## Migration Plan

No API or CRD change. After upgrading the operator:
- Existing `LoadBalancer` servers see no change.
- Existing `NodePort` servers with ports outside the node-port range turn `Degraded`/`Error` and need `gamePort`/`messagingPort` moved into the range. Before the upgrade they were not joinable anyway.
- Rollback: deploy the previous image. Services keep their last ports.
