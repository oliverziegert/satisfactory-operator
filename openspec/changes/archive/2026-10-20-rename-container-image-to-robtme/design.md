# Design

## Context

See proposal.md for motivation. The default image is defined in two places: the kubebuilder default on `SatisfactoryServerSpec.Image` (applied by the API server at create time and persisted) and a fallback in `buildPodSpec` for an empty `spec.Image`. `validateImageAllowlist` compares `spec.Image` to each allowlist entry with `==`.

Upstream re-pushed historic tags under `robtme`, so `ghcr.io/robtme/satisfactory-server:v1.9.10` and `v1.9.9` exist (verified 2026-10-19 against GHCR and Docker Hub).

## Goals / Non-Goals

**Goals:**
- Move every default and example to the new image name, without changing any logic.

**Non-Goals:**
- Changing the matching code. The spec only writes down what the code already does.
- Removing the duplicated default (CRD default + controller fallback). Both are updated in place.

## Decisions

1. **Default to `ghcr.io/robtme/satisfactory-server`, not Docker Hub.** The game server image is 8GB+ and pulled per node. GHCR avoids Docker Hub's anonymous pull rate limits, and writing the registry into the name doesn't rely on the container runtime's implicit `docker.io` prefix. Alternatives: `robtme/satisfactory-server` (smallest diff, same form as the README), `docker.io/robtme/…` (Docker Hub with the registry written out). Both were rejected over the rate limits.
2. **Keep tag `v1.9.10`.** This change only moves the image to its new name, so it can be reviewed and reverted on its own. Version bumps are deferred to a Renovate-based change.
3. **Keep exact allowlist matching.** The field pins versions. Matching on the repository alone would let `:latest` and `:dev` through. Registry spellings are not normalized, so the sample lists `ghcr.io/robtme/`, `robtme/` and `docker.io/robtme/` explicitly. Alternatives (normalizing references, aliasing `wolveix` to `robtme`, repository-level matching) were rejected as unneeded code and as changing what the field means.
4. **No migration of existing CRs.** The operator is not deployed yet. If it were, existing CRs would keep their stored `wolveix/…` image, which upstream still updates.

## Risks / Trade-offs

- [A cluster blocks or does not mirror `ghcr.io`] → `spec.image` can still point to `robtme/satisfactory-server:<tag>` on Docker Hub, or to a mirror.
- [A user writes `docker.io/robtme/…` but the policy lists only `robtme/…`] → It is rejected as non-compliant. This is intended. The sample lists all three spellings and its comment says matching is exact.
- [Upstream stops publishing `wolveix/…`] → No impact once this change lands. Nothing in the repo refers to it outside archived changes.
