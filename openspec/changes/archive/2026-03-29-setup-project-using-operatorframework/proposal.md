## Why

There is currently no Kubernetes-native way to manage Satisfactory Dedicated Server instances. Operators who want to run Satisfactory servers on Kubernetes must manually create and maintain Deployments, Services, PersistentVolumeClaims, and ConfigMaps — a tedious and error-prone process. A Kubernetes operator built with the Operator SDK provides declarative, automated lifecycle management for Satisfactory game servers, including deployment, configuration, networking, save game backup/restore, and version updates.

## What Changes

- Scaffold a new Go-based Kubernetes operator project using `operator-sdk init`
- Define Custom Resource Definitions (CRDs) for managing Satisfactory server infrastructure:
  - `SatisfactoryServer` — manages the game server pod, configuration, networking, and update lifecycle
  - `SaveGame` — manages save game backup, restore, and retention policies
  - `ClusterPolicy` — defines cluster-wide defaults for server configuration and resource limits
- Generate controller scaffolding for each CRD using `operator-sdk create api`
- Set up the project structure with Go modules, Justfile, Dockerfile, and Kustomize manifests
- Configure RBAC roles and service account for the operator

## Capabilities

### New Capabilities
- `server-lifecycle`: Manages the full lifecycle of a Satisfactory Dedicated Server instance — creation, configuration, networking (UDP/TCP ports), health checks, updates, and deletion
- `save-management`: Handles save game backup, restore, and retention policies with support for external storage backends
- `cluster-policy`: Provides cluster-wide default configuration, resource limits, and constraints that apply to all SatisfactoryServer instances

### Modified Capabilities
<!-- No existing capabilities to modify — this is a greenfield project -->

## Impact

- **New Go module**: `github.com/oliverziegert/satisfactory-operator` with operator-sdk and controller-runtime dependencies
- **New CRDs**: `SatisfactoryServer`, `SaveGame`, `ClusterPolicy` registered in the Kubernetes API
- **Kubernetes RBAC**: New ClusterRole/Role for the operator to manage pods, services, PVCs, configmaps, and custom resources
- **Container image**: New operator container image built via multi-stage Dockerfile
- **Deployment manifests**: Kustomize-based manifests for deploying the operator and CRDs to a cluster
