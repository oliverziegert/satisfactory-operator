# Design

## Context

`cmd/main.go` builds the manager with `ctrl.Options` and no cache scoping, so every informer is cluster-wide. Pods are already excluded from the cache (`DisableFor`) and read directly. That still works in namespace mode, because the reads target the server's own namespace. Leader election uses a Lease in the operator's own namespace with its own Role, which this change leaves alone. The `ClusterPolicyReconciler` lists all `SatisfactoryServer`s (`clusterpolicy_controller.go:94`) through the cached client.

## Goals / Non-Goals

**Goals:**
- Opt-in scoping with zero behaviour change when unset.

**Non-Goals:**
- RBAC manifests for namespace mode (Helm chart change).
- Reloading the namespace list at runtime. Changing it means a restart.
- Discovering namespaces by label selector.

## Decisions

- **Scope through the cache, not the reconcilers.** Set `Cache.DefaultNamespaces` from the list. Every informer, including `Owns()` and `Watches()`, and every cached `List` (including the ClusterPolicy server count) is scoped automatically, so the controllers need no code changes. The alternative, filtering in each reconciler with predicates, still needs cluster-wide RBAC to list, so it defeats the goal.
- **ClusterPolicy stays cluster-wide.** It's cluster-scoped, and controller-runtime applies no namespace restriction to cluster-scoped kinds. Add a `ByObject` entry for `ClusterPolicy` only if the test shows otherwise.
- **Flag first, env fallback.** The flag matches the existing flags. The env var lets the Helm chart or a Downward-API setup configure it without rewriting args. Parsing lives in one small function: split on `,`, trim, drop empties. It returns `nil` for cluster-wide.
- **No new kustomize overlay.** Namespace mode is a Helm-chart feature (decided in the milestone grilling). Kustomize users can still pass the flag, but they'd have to write the Roles themselves.

## Risks / Trade-offs

- [A server in an unwatched namespace is silently ignored] → Document it in the README. It's standard operator behaviour.
- [Direct (uncached) pod reads bypass cache scoping] → They only target the reconciled server's namespace, which is always watched. The Role grants `pods` get/list/watch there.
- [A watched namespace that doesn't exist] → The informer fails to list. Surface it in the logs, and the chart documents that the namespaces must exist.
