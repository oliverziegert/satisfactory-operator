# Spec Delta

## MODIFIED Requirements

### Requirement: ClusterPolicy RBAC configuration
The operator SHALL define cluster-level RBAC rules for the ClusterPolicy controller granting access to manage ClusterPolicy custom resources, read and patch SatisfactoryServer resources across all namespaces, and create Events. In namespace-scoped mode, the cluster-level rules SHALL be limited to ClusterPolicy resources and their status, and the SatisfactoryServer and Event rules SHALL be granted per watched namespace.

#### Scenario: RBAC markers
- **WHEN** the controller source code is processed by controller-gen
- **THEN** ClusterRole RBAC rules SHALL include `patch` verb for SatisfactoryServer resources and `create` verb for Events

#### Scenario: Namespace-scoped RBAC
- **WHEN** the operator is installed with watch namespaces `games`
- **THEN** its only cluster-level permissions are on `clusterpolicies` and `clusterpolicies/status`, and it can patch SatisfactoryServer resources and create Events in `games` only
