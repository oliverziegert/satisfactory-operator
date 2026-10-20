# Design

## Context

- CI has three workflows (`lint.yml`, `test.yml`, `test-e2e.yml`). Their job names `Lint`, `Tests` and `E2E Tests` are required checks on `main`. The repo allows squash merges only, with auto-merge on.
- There are no tags. `Makefile` has `VERSION ?= 0.0.1` and `IMAGE_TAG_BASE ?= ghcr.io/oliverziegert/satisfactory-operator`, and `IMG` is `$(IMAGE_TAG_BASE):v$(VERSION)`. `PLATFORMS` lists four architectures.
- `config/manager/manager.yaml` uses `image: controller:latest`, and `config/manager/kustomization.yaml` has no `images:` entry. `make build-installer` and `make deploy` set the image at build time with `kustomize edit set image`.
- `Dockerfile` already uses `TARGETOS`/`TARGETARCH`, but its builder stage runs on the target platform, so a multi-arch build would compile under QEMU.
- Renovate (`.github/renovate.json5`) extends `config:recommended` and doesn't set `semanticCommits`, so its titles look like `Update module ...`.

## Goals / Non-Goals

**Goals:**
- One workflow file holds release-please and publishing, so tag creation never depends on cross-workflow triggers.
- Nothing needs a hand-edited version string after the initial setup.

**Non-Goals:**
- Helm chart (separate change later), OLM bundle/catalog publishing, OperatorHub.
- Vulnerability scanning, cosign signatures, in-cluster signature enforcement.
- Prereleases, release branches, or backports.

## Decisions

### release-please in manifest mode, `release-type: go`
`release-please-config.json` and `.release-please-manifest.json` at the root, one package `"."`.
- The `go` type needs no version file.
- `bump-minor-pre-major: true` keeps breaking changes below 1.0.
- `"release-as": "0.1.0"` pins the first release and is removed once `v0.1.0` exists.
- `changelog-sections` lists `chore` as visible ("Miscellaneous Chores"), so `chore(deps)` entries appear in the notes (`fix(deps)` sits under "Bug Fixes"). Visibility is per type, not per scope, so other `chore` commits show up too. That's acceptable.
- *Alternative:* semantic-release. Rejected because it releases on every `fix(deps)` merge with no batching.

### Single `release.yml` with two jobs
On `push` to `main`, with `concurrency: release`:
1. **`release-please`**: mints a token with `actions/create-github-app-token` and runs `googleapis/release-please-action`. Outputs: `release_created`, `tag_name`.
2. **`publish`**: `needs: release-please`, `if: release_created`. It checks out `tag_name`, then:
   - **Image:** `docker/setup-buildx-action`, `docker/login-action` to GHCR with `GITHUB_TOKEN`, and `docker/metadata-action` with `flavor: latest=false` and tags `type=semver,pattern=v{{version}}` + `type=semver,pattern=v{{major}}.{{minor}}`, fed `value: <tag_name>`. Then `docker/build-push-action` with `platforms: linux/amd64,linux/arm64`, `sbom: true`, `provenance: mode=max`. Then `actions/attest-build-provenance` with the pushed digest and `push-to-registry: true`.
   - **Install manifest:** `make kustomize`, then `bin/kustomize build config/default > dist/install.yaml`, then `gh release upload <tag> dist/install.yaml`.
   - **Permissions:** `contents: write`, `packages: write`, `id-token: write`, `attestations: write`.

The release-please job uses the App token, so the release PR it opens triggers the required checks. The publish job uses `GITHUB_TOKEN`, since nothing downstream needs to react to it.

*Alternative:* a tag-triggered workflow. Rejected because it would need the App token to push the tag as well, and it splits the release into two places.

The workflow skips `make build-installer` and `make docker-buildx`. `build-installer` mutates the kustomization, which is pointless once it's pinned. `docker-buildx` generates `Dockerfile.cross` via `sed` and knows nothing about attestations.

### Version markers (release-please generic updater, `extra-files`)
- **Makefile:** `x-release-please-start-version` / `x-release-please-end` comment lines around `VERSION ?=`. An inline trailing comment would leave a trailing space in a `?=` value and break `IMG`.
- **`config/manager/kustomization.yaml`:** add `images: [{name: controller, newName: ghcr.io/oliverziegert/satisfactory-operator, newTag: v0.0.1}]` with an inline `# x-release-please-version` on `newTag`. YAML has no trailing-space issue.
- **README:** start/end markers as shell comments inside the install code block, around the `releases/download/vX.Y.Z/install.yaml` URL.

### Dockerfile cross-compilation
`FROM --platform=$BUILDPLATFORM golang:...@sha256:... AS builder`. The existing `GOARCH=${TARGETARCH}` then cross-compiles natively. The final distroless stage has no `RUN`, so no QEMU is needed anywhere. Renovate's dockerfile manager handles `--platform` flags.

### PR-title check
`.github/workflows/pr-title.yml` runs on `pull_request_target` (types `opened`, `edited`, `synchronize`, `reopened`) with job name `PR Title`, using `amannn/action-semantic-pull-request`. It never checks out PR code, so `pull_request_target` is safe and also works for fork PRs. The repo setting `squash_merge_commit_title=PR_TITLE` makes the title the commit.

### Renovate commit types
- Set `semanticCommits: "enabled"` and a top-level `semanticCommitType: "chore"`.
- Add a packageRule `matchManagers: ["gomod", "dockerfile"]` → `semanticCommitType: "fix"`.
- After it, a rule for `github.com/onsi/**` → `chore`. These are the only test-only modules; envtest ships inside controller-runtime.
- The scope is `deps` by default.
- Explicit rules instead of relying on `config:recommended`'s `:semanticPrefixFixDepsChoreOthers`, which keys on depType and would type Dockerfile images as `chore`.

## Risks / Trade-offs

- [`make deploy`/`build-installer` run `kustomize edit set image` locally and may drop the `# x-release-please-version` comment if the developer commits that file] → Verify comment preservation during implementation. If kustomize strips it, have those targets restore the file afterwards (`git checkout`) or switch them to a temporary overlay.
- [`newTag` points at a non-existent `v0.0.1` image until the first release] → Acceptable, since no install path is documented until `v0.1.0` exists. The README snippet is added alongside it.
- [The GHCR package is created private on first push, so the first `install.yaml` pulls fail for anonymous users] → One-time manual visibility change, listed in the tasks.
- [The App private key leaks or expires] → It doesn't expire. Rotate it in the App settings. The App is installed on this repo only, with contents + pull-requests write.
- [`chore` changelog section also shows non-dependency chores] → Accepted. Splitting by scope isn't supported by release-please.
- [A visible `chore` section makes chore-only commits open a release PR (release-please skips only when the changelog would be empty)] → Accepted. The release PR is only a proposal; nothing is published until a human merges it.
- [Release PR merged while `main` has moved on] → release-please rebases or regenerates the PR on each push, and `concurrency: release` serialises runs.

## Migration Plan

1. Merge this change. The first push to `main` opens the `0.1.0` release PR.
2. Before merging that PR, do the manual setup: App + secrets, required `PR Title` check, `PR_TITLE` squash setting.
3. Merge the release PR, then make the GHCR package public.
4. Remove `release-as` in a follow-up `chore:` PR.

Rollback: delete `release.yml`. Tags and images already published stay valid.
