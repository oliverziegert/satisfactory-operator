# Spec Delta

## ADDED Requirements

### Requirement: Backup Job image
The chart SHALL set the manager's `RELATED_IMAGE_BACKUP` environment variable to the same image reference as the manager container, built from `manager.image.repository` and `manager.image.tag` (defaulting to `Chart.AppVersion`, as for the manager).

#### Scenario: Default values
- **WHEN** the chart is rendered with default values
- **THEN** `RELATED_IMAGE_BACKUP` equals the manager container's `image`

#### Scenario: Overridden tag
- **WHEN** the chart is rendered with `manager.image.tag=v9.9.9`
- **THEN** both the manager image and `RELATED_IMAGE_BACKUP` end in `:v9.9.9`

#### Scenario: Regeneration
- **WHEN** `make helm` regenerates the chart
- **THEN** the env var is still present and templated, and the drift check passes
