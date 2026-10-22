# Tasks

## 1. Status API

- [x] 1.1 In `api/v1alpha1/satisfactoryserver_types.go`, change `ConnectedPlayers` to `*int32` with `omitempty` and remove `ServerVersion`; run `make manifests generate` and verify the CRD has no `serverVersion` and `go build ./...` fails only at the controller usages fixed in 2.x

## 2. Controller

- [x] 2.1 Make `probeResult.NumPlayers` a `*int32` set with `ptr.To` only when `QueryServerState` succeeds, remove `probeResult.ServerVersion`, and in `setGameConditions` copy `probe.NumPlayers` to `status.connectedPlayers` (dropping the `ServerVersion` lines) and set it to `nil` in the `probe == nil` branch; verify with `go build ./...`
- [x] 2.2 Add `const statusPollInterval = 60 * time.Second` and return `ctrl.Result{RequeueAfter: statusPollInterval}` instead of the final `ctrl.Result{}` in `Reconcile`; verify with `go build ./...`
- [x] 2.3 Update `internal/controller/satisfactoryserver_probe_test.go`: the success probe returns `NumPlayers=2`, a 0-player response gives `ptr(0)`, auth error and connection failure give `nil`, `setGameConditions` copies `3`, and the nil-probe case clears a previously set count; verify `go test ./internal/controller/ -run 'TestProbeGameServer|TestSetGameConditions' -v` passes
- [x] 2.4 In `CLAUDE.md`, drop "version" from the probe description (Architecture, step 4); verify `grep -n version CLAUDE.md` no longer mentions the probe

## 3. Integration checks

- [x] 3.1 Run `make lint test` and verify both pass
- [x] 3.2 On the Talos cluster, deploy the branch build with a running server: verify `kubectl get satisfactoryserver` shows `0` with no players, shows `1` within about 60s of a player joining with no annotation touch, and returns to `0` after the player leaves

## Workflow follow-up

- Open the PR titled `fix: re-probe running servers and report unknown player count` with `Fixes #40`.
- Archive the change after the PR is merged.
