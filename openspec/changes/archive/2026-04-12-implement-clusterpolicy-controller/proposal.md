## Why

The ClusterPolicy controller is a stub. It doesn't enumerate SatisfactoryServer resources, apply defaults, or enforce constraints. The CRD already defines fields for `defaultResourceLimits`, `defaultStorageClassName`, `defaultServerConfig`, `allowedImageVersions`, and `maxServersPerNamespace`, but none of these have any effect. Without enforcement, cluster administrators have no way to set guardrails on game server deployments.

**No dependencies** — this operates at the Kubernetes resource level, not the game API.

## What Changes

- On reconciliation, list all SatisfactoryServer resources across all namespaces
- For each server missing explicit values, apply defaults from `defaultServerConfig` and `defaultResourceLimits` (patch the server spec)
- Validate servers against `allowedImageVersions` — set `Enforced=False` and emit an Event if a server uses a disallowed image
- Enforce `maxServersPerNamespace` — set `Enforced=False` and emit Events when the limit is exceeded
- Populate `status.serversGoverned` with the count of SatisfactoryServer resources across all namespaces
- Update `status.lastAppliedTime` on each reconciliation
- Watch SatisfactoryServer resources to re-reconcile the policy when servers are created or updated

## Capabilities

### New Capabilities

- `policy-enforcement`: The controller enforces ClusterPolicy constraints and applies defaults to SatisfactoryServer resources across all namespaces.

### Modified Capabilities

- `cluster-policy`: The controller moves from stub to functional. Conditions now reflect actual enforcement state. `serversGoverned` and `lastAppliedTime` are populated.

## Impact

- `internal/controller/clusterpolicy_controller.go` — full Reconcile implementation replacing the stub
- `internal/controller/clusterpolicy_helpers.go` — new file with default application and validation helpers
- `config/rbac/` — may need patch permissions for SatisfactoryServer resources