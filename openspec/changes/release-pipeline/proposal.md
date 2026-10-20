# Proposal

## Why

The operator has no releases. There are no tags, the operator image is never published, and installing it means cloning the repo and running `make deploy` with a self-built image. Automation is a key goal. Releases should follow from Conventional Commits with one human approval step, and no hand-run versioning, tagging or publishing.

## What Changes

- release-please keeps an open release PR on `main` with the next version and CHANGELOG. Merging that PR creates the `vX.Y.Z` tag and the GitHub Release, and the same workflow then builds and publishes.
- The operator image is published to `ghcr.io/oliverziegert/satisfactory-operator` for `linux/amd64` and `linux/arm64`, tagged `vX.Y.Z` and `vX.Y`. There is no `latest` tag, and pushes to `main` publish no images.
- The image carries buildx SBOM and provenance attestations, plus a GitHub artifact attestation that `gh attestation verify` can check.
- `install.yaml` is attached to every GitHub Release, pinned to the released image.
- The release PR bumps the version in three places: the Makefile `VERSION`, the image `newTag` in `config/manager/kustomization.yaml` (newly pinned to the GHCR image), and the README install URL.
- Versioning starts at `v0.1.0`. While below 1.0, breaking changes bump the minor version.
- Renovate writes Conventional Commits. Updates to anything that ships in the image use `fix(deps)` and trigger a release. CI and tooling updates use `chore(deps)` and only appear in the changelog.
- PR titles must be Conventional Commits, enforced by a required check. Squash merges use the PR title as the commit title.
- The Dockerfile builder stage runs on the build platform and cross-compiles, and the Makefile `PLATFORMS` default becomes `linux/amd64,linux/arm64`.
- release-please authenticates with a GitHub App token, so the required checks run on release PRs.

## Capabilities

### New Capabilities
- `release-automation`: versioning, tagging, and publishing of the operator image and install manifest, plus the commit-message conventions that drive them.

### Modified Capabilities
- `dependency-updates`: Renovate commits and PR titles become Conventional Commits, typed by whether the update ships in the operator image.

## Impact

- New: `.github/workflows/release.yml`, `.github/workflows/pr-title.yml`, `release-please-config.json`, `.release-please-manifest.json`, `CHANGELOG.md` (generated).
- Changed: `Dockerfile`, `Makefile` (`VERSION` marker, `PLATFORMS`), `config/manager/kustomization.yaml`, `README.md`, `CLAUDE.md`, `.github/renovate.json5`.
- Repository settings: squash commit title set to `PR_TITLE`, and the PR-title check added to the required status checks on `main`.
- Manual one-time steps: create and install a GitHub App and store its ID and private key as secrets, and make the GHCR package public after the first release.
