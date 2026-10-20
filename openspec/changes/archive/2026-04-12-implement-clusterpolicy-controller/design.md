## Context

The ClusterPolicy CRD is cluster-scoped and defines defaults and constraints for SatisfactoryServer resources. The controller currently sets `Ready=True` and `Enforced=True` unconditionally. The CRD spec fields (`defaultResourceLimits`, `defaultStorageClassName`, `defaultServerConfig`, `allowedImageVersions`, `maxServersPerNamespace`) are defined but have no effect.

## Goals / Non-Goals

**Goals:**

- Apply default values to SatisfactoryServer resources that don't set them explicitly
- Validate image references against an allowlist
- Enforce per-namespace server count limits
- Report compliance status via conditions and events

**Non-Goals:**

- Mutating webhook for admission-time enforcement (future work — the controller applies defaults at reconcile time)
- Multiple ClusterPolicy resources (only one active policy is supported)
- Namespace-scoped policies
- Blocking server creation (the controller reports non-compliance, it does not reject resources)

## Decisions

### 1. Reconcile-time defaults, not admission-time

The controller patches SatisfactoryServer specs during reconciliation. This means a newly created server without defaults will briefly exist before the policy applies.

**Alternative considered:** Mutating admission webhook. Rejected for now because it adds deployment complexity (cert-manager, webhook configuration) and a runtime dependency. Reconcile-time is simpler and sufficient for v1alpha1.

### 2. Single active ClusterPolicy

If multiple ClusterPolicy resources exist, the controller uses the oldest one (by creation timestamp) and sets `Ready=False` on the others with reason `MultiplePolicesDetected`. This avoids conflicting defaults.

### 3. Defaults applied only when spec fields are zero-valued

The controller checks whether a SatisfactoryServer field is zero-valued (nil pointer, 0 integer, empty string) before applying a default. If the user explicitly set a value, even if it matches the default, it is not overwritten. This is checked via the field's zero value, not by comparing to the ClusterPolicy default.

### 4. Non-compliance reported, not blocked

When a server uses a disallowed image or exceeds the namespace limit, the controller:
- Sets `Enforced=False` on the ClusterPolicy with a message listing non-compliant servers
- Emits a Warning Event on the non-compliant SatisfactoryServer
- Does NOT delete or modify the non-compliant server

This is a soft enforcement model. Hard enforcement via admission webhooks can be added later.

### 5. Watch SatisfactoryServer for re-reconciliation

The controller watches SatisfactoryServer resources across all namespaces. Any create/update/delete triggers a re-reconciliation of the ClusterPolicy. This is efficient because there is only one (or very few) ClusterPolicy resources.

## Risks / Trade-offs

**[Risk] Race between SatisfactoryServer controller and ClusterPolicy controller** → Both may try to update the SatisfactoryServer spec simultaneously. Mitigated by using `Patch` with `MergeFrom` (optimistic concurrency). On conflict, the controller retries on the next reconciliation.

**[Risk] Default application triggers another reconciliation of SatisfactoryServer** → Expected and harmless. The SatisfactoryServer controller reconciles the StatefulSet with the updated spec.

**[Trade-off] No admission-time enforcement** → Servers can briefly exist in a non-compliant state. Acceptable for alpha. The condition and event system makes non-compliance visible immediately.
