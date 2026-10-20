# Spec Delta

## MODIFIED Requirements

### Requirement: Default container image
The operator SHALL use `ghcr.io/robtme/satisfactory-server:v1.9.10` as the default container image for SatisfactoryServer resources when no image is specified in the CRD spec. The container spec SHALL include liveness and readiness probes in addition to ports, environment variables, volume mounts, and resource requirements.

#### Scenario: No image specified
- **WHEN** a SatisfactoryServer resource is created without an `image` field
- **THEN** the operator SHALL create a Pod using `ghcr.io/robtme/satisfactory-server:v1.9.10`

#### Scenario: Custom image specified
- **WHEN** a SatisfactoryServer resource specifies `image: robtme/satisfactory-server:v1.8.7`
- **THEN** the operator SHALL create a Pod using the specified image and tag

#### Scenario: Container includes health probes
- **WHEN** a SatisfactoryServer Pod is created
- **THEN** the container spec SHALL include a readiness probe and a liveness probe in addition to ports, environment variables, volume mounts, and resource requirements
