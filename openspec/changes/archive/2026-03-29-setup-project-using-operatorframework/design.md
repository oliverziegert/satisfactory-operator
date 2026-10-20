## Context

This is a greenfield Kubernetes operator project for managing Satisfactory Dedicated Server instances. The project will be scaffolded using the Operator SDK (Go) which provides controller-runtime integration, CRD generation, RBAC scaffolding, and standard project layout. There is no existing codebase — all structure will be generated.

Satisfactory Dedicated Server runs as a single Linux process, uses UDP port 7777 (game traffic), TCP port 7777 (server query) and TCP port 8888 (reliable messaging), and stores save data on disk. It supports headless configuration via CLI flags and environment variables. The server can be managed via a REST-like Server API on a configurable port.

## Goals / Non-Goals

**Goals:**
- Scaffold a well-structured operator-sdk Go project with standard tooling (Justfile, Dockerfile, Kustomize)
- Define three CRDs: `SatisfactoryServer`, `SaveGame`, `ClusterPolicy` with proper API versioning (v1alpha1)
- Generate controller stubs for each CRD with reconciliation loops
- Set up RBAC with least-privilege access for the operator
- Establish the project foundation so subsequent changes can implement actual reconciliation logic

**Non-Goals:**
- Implementing full reconciliation logic (this change only scaffolds the project and defines the API types)
- Helm chart packaging (Kustomize is the primary deployment method for now)
- Multi-cluster support
- Automated testing infrastructure beyond what operator-sdk generates
- Integration with external backup storage (S3, GCS) — that's a future enhancement on top of SaveGame

## Decisions

### 1. API Group and Version: `satisfactory.oliverziegert.github.com/v1alpha1`

Use `v1alpha1` as the initial API version since the API surface is not yet stable. The API group follows the standard reverse-domain convention matching the repository owner.

### 2. Three separate CRDs rather than a single monolithic CRD

Splitting into `SatisfactoryServer`, `SaveGame`, and `ClusterPolicy` follows the Kubernetes resource model — each resource has a single responsibility and can be managed independently. SaveGames can exist independently of a server (e.g., for migration). ClusterPolicy is cluster-scoped while servers are namespaced.

### 3. Operator SDK over raw kubebuilder

Operator SDK wraps kubebuilder and adds OLM integration, scorecard testing, and bundle packaging. Even though we won't use OLM immediately, the additional tooling comes at no cost and provides a growth path.

### 4. Kustomize for deployment manifests

Operator SDK generates Kustomize bases by default. We'll use this standard approach with overlays for development and production.

### 5. Resource scoping

- `SatisfactoryServer`: **Namespaced** — allows multi-tenant clusters, RBAC per namespace
- `SaveGame`: **Namespaced** — belongs to a server's namespace
- `ClusterPolicy`: **Cluster-scoped** — defines cluster-wide defaults

## Risks / Trade-offs

- **[API instability]** v1alpha1 signals instability; users must expect breaking changes → Document clearly, provide migration guides when graduating to v1beta1
- **[Scope creep in CRD design]** Defining three CRDs upfront may be over-engineering for initial needs → Keep type definitions minimal; add fields as requirements solidify
- **[Operator SDK version lock-in]** Operator SDK versions can have breaking changes → Pin to a specific version in go.mod; upgrade deliberately
- **[SaveGame complexity]** Independent SaveGame resources add reconciliation complexity (cross-resource references) → Start with simple owner references; SaveGame controller watches for parent SatisfactoryServer