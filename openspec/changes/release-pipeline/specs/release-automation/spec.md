# Spec Delta

## Purpose

Turns Conventional Commits on `main` into versioned, attested releases of the operator image and install manifest, with one human approval step per release.

## ADDED Requirements

### Requirement: Release PR tracks unreleased changes
While `main` contains release-relevant commits (any type with a visible changelog section: `feat`, `fix`, `perf`, `revert`, `chore`) that are not in any release yet, the repository SHALL have exactly one open release PR. That PR carries the next version and the CHANGELOG entries for those commits.

#### Scenario: Fix merged to main
- **WHEN** a `fix:` commit lands on `main` after the latest release
- **THEN** the release PR is opened or updated, proposing the next patch version and listing the fix in the CHANGELOG

#### Scenario: Only hidden-type commits
- **WHEN** only `ci`, `test`, `docs`, `build`, `style` or `refactor` commits have landed since the latest release
- **THEN** no release PR is opened

#### Scenario: Only chore commits
- **WHEN** only `chore` commits have landed since the latest release
- **THEN** the release PR is opened or updated, proposing the next patch version, and nothing is published until it is merged

### Requirement: Merging the release PR releases
Merging the release PR SHALL create the `vX.Y.Z` git tag and a GitHub Release, and SHALL publish all release artifacts, with no further manual steps.

#### Scenario: Release PR merged
- **WHEN** the maintainer merges the release PR for `0.3.1`
- **THEN** the tag `v0.3.1`, a GitHub Release `v0.3.1`, the image and `install.yaml` all exist

### Requirement: Release PR runs required checks
The release PR SHALL trigger the same required status checks as any other PR into `main`.

#### Scenario: Release PR opened
- **WHEN** the release PR is opened or updated
- **THEN** Lint, Tests, E2E Tests and the PR-title check run on it

### Requirement: Semantic version bumps
Versions SHALL follow SemVer from commit types. While below 1.0, `feat` bumps the minor version, `fix` bumps the patch, and breaking changes bump the minor version. The first release SHALL be `v0.1.0`.

#### Scenario: First release
- **WHEN** the first release PR is merged
- **THEN** the released version is `v0.1.0`

#### Scenario: Feature before 1.0
- **WHEN** the latest release is `v0.3.1` and a `feat:` commit is unreleased
- **THEN** the release PR proposes `v0.4.0`

#### Scenario: Breaking change before 1.0
- **WHEN** the latest release is `v0.3.1` and a `feat!:` commit is unreleased
- **THEN** the release PR proposes `v0.4.0`, not `v1.0.0`

### Requirement: Multi-arch operator image
Each release SHALL publish the operator image to `ghcr.io/oliverziegert/satisfactory-operator` as a multi-platform image for `linux/amd64` and `linux/arm64`.

#### Scenario: Pull on arm64
- **WHEN** an arm64 node pulls the released image
- **THEN** it receives a native arm64 image

### Requirement: Image tags
A release `vX.Y.Z` SHALL push the image tags `vX.Y.Z` and `vX.Y`. It SHALL NOT push `latest`. Pushes to `main` that are not releases SHALL NOT publish any image.

#### Scenario: Patch release
- **WHEN** `v0.3.2` is released
- **THEN** tags `v0.3.2` and `v0.3` both point to the new image, and `v0.3.1` is unchanged

#### Scenario: Ordinary merge to main
- **WHEN** a non-release commit is pushed to `main`
- **THEN** no image is pushed to the registry

### Requirement: Image attestations
Each released image SHALL carry an SBOM and build provenance in the registry. It SHALL also have a GitHub artifact attestation that `gh attestation verify` can check against this repository.

#### Scenario: Verify a release
- **WHEN** a user runs `gh attestation verify oci://ghcr.io/oliverziegert/satisfactory-operator:v0.3.1 -R oliverziegert/satisfactory-operator`
- **THEN** verification succeeds

### Requirement: Install manifest asset
Each GitHub Release SHALL include an `install.yaml` asset with the CRDs, RBAC and controller Deployment, referencing the image of that release.

#### Scenario: Install from release
- **WHEN** a user applies `https://github.com/oliverziegert/satisfactory-operator/releases/download/v0.3.1/install.yaml`
- **THEN** the controller Deployment runs `ghcr.io/oliverziegert/satisfactory-operator:v0.3.1`

### Requirement: Version references stay current
The release PR SHALL update every in-repo version reference to the new version: the Makefile `VERSION` default, the controller image tag in the kustomize config, and the README install instructions.

#### Scenario: Kustomize remote install at a tag
- **WHEN** a user runs `kubectl apply -k github.com/oliverziegert/satisfactory-operator/config/default?ref=v0.3.1`
- **THEN** the controller Deployment runs `ghcr.io/oliverziegert/satisfactory-operator:v0.3.1`

#### Scenario: Local build defaults
- **WHEN** a developer runs `make docker-build` on the `v0.3.1` tag without `IMG`
- **THEN** the image is tagged `ghcr.io/oliverziegert/satisfactory-operator:v0.3.1`

### Requirement: Conventional PR titles
A PR into `main` SHALL NOT be mergeable unless its title is a valid Conventional Commit. Squash merges SHALL use the PR title as the commit title.

#### Scenario: Non-conventional title
- **WHEN** a PR titled `Update stuff` targets `main`
- **THEN** the required PR-title check fails and the PR cannot be merged

#### Scenario: Multi-commit PR squash-merged
- **WHEN** a PR titled `feat: add backup restore` with several commits is squash-merged
- **THEN** the commit on `main` is titled `feat: add backup restore`
