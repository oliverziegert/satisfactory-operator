# Proposal

## Why

The operator watches every namespace through a ClusterRole that grants cluster-wide `secrets` create/get/list/watch/update and `statefulsets` create/delete. On a shared cluster that's far more access than a game-server operator needs. A namespace-scoped mode cuts that down to the namespaces that actually host servers. It's also the prerequisite for the namespace-scoped Helm install in `add-helm-chart`.

## What Changes

- New manager flag `--watch-namespaces` (comma-separated). The `WATCH_NAMESPACES` env var is the fallback when the flag isn't set.
- Empty or unset means cluster-wide, which is today's behaviour. Existing `install.yaml` and kustomize installs are unchanged.
- When set, the operator caches and reconciles `SatisfactoryServer`, `SaveGame` and their owned objects only in the listed namespaces. It ignores resources in other namespaces.
- `ClusterPolicy` stays watched cluster-wide in both modes. In namespace mode, its defaults and constraints apply only to servers in watched namespaces.
- No RBAC manifests change here. Namespace-scoped RBAC is rendered by the Helm chart (`add-helm-chart`).

## Capabilities

### New Capabilities
- `namespace-scoping`: which namespaces the operator watches and reconciles, and how it's configured.

### Modified Capabilities
- (none; `cluster-policy` RBAC wording changes in `add-helm-chart`, where namespace-scoped RBAC is introduced)

## Impact

- `cmd/main.go`: flag/env parsing, `cache.Options` for the manager.
- New unit test for parsing and cache options.
- README: document the flag and the "servers in unwatched namespaces are ignored" behaviour.
- No API (CRD) changes, no new dependencies.
