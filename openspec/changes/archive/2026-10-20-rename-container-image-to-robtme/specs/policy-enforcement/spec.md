# Spec Delta

## MODIFIED Requirements

### Requirement: Validate allowed image versions
The ClusterPolicy controller SHALL validate SatisfactoryServer image references against `allowedImageVersions`. A reference SHALL be compliant only if it exactly equals an allowlist entry, including registry, repository and tag; different spellings of the same image (e.g. `robtme/…` vs `docker.io/robtme/…`) are distinct. Non-compliant servers SHALL be reported but not modified.

#### Scenario: Image in allowlist
- **WHEN** a SatisfactoryServer uses `image: ghcr.io/robtme/satisfactory-server:v1.9.10` and `allowedImageVersions` contains `ghcr.io/robtme/satisfactory-server:v1.9.10`
- **THEN** the server SHALL be considered compliant

#### Scenario: Same image under a different registry spelling
- **WHEN** a SatisfactoryServer uses `image: docker.io/robtme/satisfactory-server:v1.9.10` and `allowedImageVersions` contains only `robtme/satisfactory-server:v1.9.10`
- **THEN** the server SHALL be considered non-compliant

#### Scenario: Same repository with a different tag
- **WHEN** a SatisfactoryServer uses `image: ghcr.io/robtme/satisfactory-server:latest` and `allowedImageVersions` contains only `ghcr.io/robtme/satisfactory-server:v1.9.10`
- **THEN** the server SHALL be considered non-compliant

#### Scenario: Image not in allowlist
- **WHEN** a SatisfactoryServer uses an image not in `allowedImageVersions`
- **THEN** the controller SHALL emit a Warning Event on the SatisfactoryServer and include it in the `Enforced=False` condition message on the ClusterPolicy

#### Scenario: Empty allowlist
- **WHEN** `allowedImageVersions` is empty or nil
- **THEN** all images SHALL be considered compliant (no restriction)
