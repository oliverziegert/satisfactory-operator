## 1. RBAC

- [x] 1.1 Add RBAC markers for `patch` on SatisfactoryServer resources and `create` on Events to `clusterpolicy_controller.go`
- [x] 1.2 Run `make manifests` and verify updated ClusterRole

## 2. Helper Functions

- [x] 2.1 Create `internal/controller/clusterpolicy_helpers.go` with helper `applyDefaults(server, policy) bool` that patches zero-valued fields from ClusterPolicy defaults, returns true if any field was changed
- [x] 2.2 Add helper `validateImageAllowlist(server, allowedImages) bool` that checks the server image against the allowlist
- [x] 2.3 Add helper `countServersPerNamespace(servers) map[string]int` that groups SatisfactoryServer resources by namespace
- [x] 2.4 Add helper `findActivePolicy(policies) (*ClusterPolicy, []*ClusterPolicy)` that returns the oldest policy and the list of inactive ones

## 3. Controller Reconciliation

- [x] 3.1 Rewrite `ClusterPolicyReconciler.Reconcile` with full logic: list all ClusterPolicies → determine active policy → list all SatisfactoryServers → apply defaults (patch) → validate constraints → collect violations → set Enforced condition → set Ready condition → populate serversGoverned and lastAppliedTime → update status
- [x] 3.2 Handle multiple policies: set `Ready=False` with reason `MultiplePoliciesDetected` on inactive policies
- [x] 3.3 Emit Warning Events on non-compliant SatisfactoryServer resources (disallowed image, namespace limit exceeded)

## 4. Watch Setup

- [x] 4.1 Update `SetupWithManager` to watch SatisfactoryServer resources across all namespaces so policy is re-reconciled on server create/update/delete

## 5. Tests

- [x] 5.1 Add unit tests for `applyDefaults` — defaults applied, explicit values preserved, no defaults configured
- [x] 5.2 Add unit tests for `validateImageAllowlist` — allowed, disallowed, empty allowlist
- [x] 5.3 Add unit tests for `countServersPerNamespace` and namespace limit enforcement
- [x] 5.4 Add unit tests for `findActivePolicy` — single policy, multiple policies
- [x] 5.5 Add integration tests for full Reconcile loop with envtest — defaults applied, violations detected, conditions set
- [x] 5.6 Run `make test` and verify all tests pass

## 6. Samples

- [x] 6.1 Update `config/samples/satisfactory_v1alpha1_clusterpolicy.yaml` with realistic defaults and constraints
