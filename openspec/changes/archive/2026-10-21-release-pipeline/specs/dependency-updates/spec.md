# Spec Delta

## ADDED Requirements

### Requirement: Dependency updates are typed by release impact
Renovate PR titles and commits SHALL be Conventional Commits. Updates to dependencies that ship in the operator image (Go modules outside the test-only set, the Go builder image, the distroless base image) SHALL use type `fix(deps)`. All other updates SHALL use type `chore(deps)`.

#### Scenario: Runtime Go module update
- **WHEN** Renovate updates `sigs.k8s.io/controller-runtime`
- **THEN** the PR title starts with `fix(deps):` and the update triggers a release PR

#### Scenario: Base image digest update
- **WHEN** Renovate updates the distroless base image digest
- **THEN** the PR title starts with `fix(deps):`

#### Scenario: Test library update
- **WHEN** Renovate updates `github.com/onsi/gomega`
- **THEN** the PR title starts with `chore(deps):`, and the update alone may open a release PR but publishes nothing until that PR is merged

#### Scenario: CI or tooling update
- **WHEN** Renovate updates a GitHub Action or a Makefile-pinned tool
- **THEN** the PR title starts with `chore(deps):`

### Requirement: Dependency changes listed in release notes
Every dependency update since the last release, `fix(deps)` and `chore(deps)` alike, SHALL appear in the CHANGELOG of the next release.

#### Scenario: Chore update rides along
- **WHEN** a `chore(deps)` update and a `fix:` commit landed since the last release
- **THEN** the next release's CHANGELOG lists both
