# Spec Delta

## ADDED Requirements

### Requirement: Helm chart artifact
Each release `vX.Y.Z` SHALL publish the Helm chart as an OCI artifact to `oci://ghcr.io/oliverziegert/charts/satisfactory-operator` with chart `version` `X.Y.Z` and `appVersion` `vX.Y.Z`. Pushes to `main` that are not releases SHALL NOT publish a chart.

#### Scenario: Chart pull after release
- **WHEN** release `v0.3.0` has been published
- **THEN** `helm pull oci://ghcr.io/oliverziegert/charts/satisfactory-operator --version 0.3.0` succeeds and its default image tag is `v0.3.0`

## MODIFIED Requirements

### Requirement: Version references stay current
The release PR SHALL update every in-repo version reference to the new version: the Makefile `VERSION` default, the controller image tag in the kustomize config, the Helm chart's `version` and `appVersion`, and the README install instructions.

#### Scenario: Kustomize remote install at a tag
- **WHEN** a user runs `kubectl apply -k github.com/oliverziegert/satisfactory-operator/config/default?ref=v0.3.1`
- **THEN** the controller Deployment runs `ghcr.io/oliverziegert/satisfactory-operator:v0.3.1`

#### Scenario: Local build defaults
- **WHEN** a developer runs `make docker-build` on the `v0.3.1` tag without `IMG`
- **THEN** the image is tagged `ghcr.io/oliverziegert/satisfactory-operator:v0.3.1`

#### Scenario: Chart from a tag checkout
- **WHEN** a user runs `helm install sat ./dist/chart` on the `v0.3.1` tag
- **THEN** the controller Deployment runs `ghcr.io/oliverziegert/satisfactory-operator:v0.3.1`
