# Tasks

## 1. Watch-namespaces configuration

- [x] 1.1 Add `--watch-namespaces` flag and `WATCH_NAMESPACES` fallback in `cmd/main.go`, with a parse function (split, trim, drop empties, `nil` = cluster-wide). Verify with a table test in `cmd/main_test.go` covering flag, env, flag-over-env, whitespace and empty input.
- [x] 1.2 Set `Cache.DefaultNamespaces` on the manager when the list is non-empty, and log the effective mode at startup. Verify with `make build` and `make lint`, and confirm the startup log line with `make run ARGS=--watch-namespaces=games`, or `go run ./cmd/main.go --watch-namespaces=games` if `make run` has no `ARGS` passthrough.

## 2. Behaviour verification

- [x] 2.1 Against a local cluster running `go run ./cmd/main.go --watch-namespaces=games`, verify that a server in `games` gets a StatefulSet, a server in `other` gets nothing, and a `ClusterPolicy` still reconciles (status `Ready`) and patches defaults onto the `games` server only. Automated coverage for this lands with the chart e2e in `add-helm-chart`.
- [x] 2.2 Document the flag, env var and "unwatched namespaces are ignored" behaviour in `README.md`. Verify that the documented command works as written.

## Workflow follow-up

- Open the PR titled `feat: add namespace-scoped watch mode`.
- Archive the change after merge.
