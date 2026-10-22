# Design

## Context

`Reconcile` probes the game API only while it runs. Its last statement is `return ctrl.Result{}, nil`. The controller watches the SatisfactoryServer and its owned StatefulSet, Services and Secret without predicates, and players joining doesn't change any of them. The probe address is the in-cluster DNS name of pod 0, so outside a cluster a reconcile never reaches the reachable path.

## Goals / Non-Goals

**Goals:**
- Keep the status of a running server at most about 60 seconds old.
- Make the player count say "unknown" instead of `0` when it isn't known.

**Non-Goals:**
- A configurable poll interval.
- Reading the game version (for example, through the UDP Lightweight Query API).
- A test seam for the probe address, to test the requeue in a unit test.

## Decisions

- **Fixed 60s requeue at the end of `Reconcile`** (`statusPollInterval`, following `saveGameRetryInterval` in `savegame_controller.go`). The final `return ctrl.Result{}, nil` becomes `RequeueAfter: statusPollInterval`, so no path that reaches the end of a reconcile leaves a server without a next probe. The 5s and 30s requeues before it are unchanged. Alternatives: a spec field (nobody asked for tuning), or requeueing only when `probe != nil` (an extra condition, and the "StatefulSet not cached yet" case would then rely on a watch event).
- **`ConnectedPlayers *int32` with `omitempty`.** `probeResult.NumPlayers` also becomes `*int32` and is set (with `ptr.To`, from `k8s.io/utils/ptr`, already a dependency) only when `QueryServerState` succeeds. `setGameConditions` copies it as is and sets `nil` in the `probe == nil` branch. kubectl prints a missing integer column as blank. Alternative: drop `omitempty` on `int32`. That is smaller, but "unknown" would print as `0`.
- **Remove `ServerVersion`** from the status type, `probeResult` and `setGameConditions`. The HTTPS API has no version, and the image tag doesn't reflect self-updates.

## Risks / Trade-offs

- [Every running server makes up to three game API calls per minute: token verification, `QueryServerState` and the claim check] → negligible for single-instance game servers. Revisit if the claim check shows up in server logs as noise.
- [Status writes don't trigger a reconcile when nothing changed, because the API server skips no-op updates. A changed player count triggers one extra immediate reconcile] → harmless: that reconcile ends with the same 60s requeue.
- [The 60s requeue has no unit test, because the probe address can't be redirected] → it is a one-line constant. It is verified manually on Talos, see tasks.md.
- [Removing `status.serverVersion` from a v1alpha1 CRD] → the field was never set, so no stored object loses data.
