# Proposal

## Why

The upstream maintainer of the community game server image renamed their account from `wolveix` to `robtme` (see https://github.com/robtme/satisfactory-server). The image is now published as `robtme/satisfactory-server` on Docker Hub and `ghcr.io/robtme/satisfactory-server` on GHCR. The old `wolveix/satisfactory-server` name still receives updates for now, but without a stated end date, so the operator should default to the canonical name before its first deployment.

## What Changes

- Default container image changes from `wolveix/satisfactory-server:v1.9.10` to `ghcr.io/robtme/satisfactory-server:v1.9.10` (CRD default and controller fallback). The tag is unchanged.
- The `allowedImageVersions` requirement states explicitly that matching is an exact string comparison of the full image reference (registry, repository, tag). Behavior is unchanged; only the spec wording is.
- The sample ClusterPolicy allowlists the image under all three spellings (`ghcr.io/robtme/…`, `robtme/…`, `docker.io/robtme/…`) for `v1.9.10` and `v1.9.9`.
- Samples, tests, code comments and `CLAUDE.md` refer to the new name.

Non-goals:
- Bumping the image version (e.g. to `v1.9.11`). That is left to a later change introducing Renovate.
- Migrating existing SatisfactoryServer resources. The operator has not been deployed yet.
- Treating `wolveix/…` and `robtme/…`, or different registry spellings, as equivalent.
- Repository-level (tag-agnostic) allowlist matching.
- Editing archived changes under `openspec/changes/archive/`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities
- `container-image-config`: default container image changes to `ghcr.io/robtme/satisfactory-server:v1.9.10`.
- `policy-enforcement`: allowlist requirement states exact matching on the full image reference; scenarios use the new image name.

## Impact

- `api/v1alpha1/satisfactoryserver_types.go` (kubebuilder default and doc comment), regenerated CRD in `config/crd/bases/`
- `internal/controller/satisfactoryserver_helpers.go` (fallback image, env-var comment)
- `config/samples/` (SatisfactoryServer and ClusterPolicy samples)
- `internal/controller/*_test.go` (image string assertions)
- `CLAUDE.md`
- Runtime: new servers pull from `ghcr.io` instead of Docker Hub. Clusters with registry allowlists or mirrors must permit `ghcr.io`.
