# Tasks

## 1. Build inputs

- [x] 1.1 Change the Dockerfile builder stage to `FROM --platform=$BUILDPLATFORM golang:...` (digest kept) and verify `docker buildx build --platform linux/amd64,linux/arm64 .` succeeds without QEMU running the Go build
- [x] 1.2 Set the Makefile `PLATFORMS` default to `linux/amd64,linux/arm64` and wrap `VERSION ?=` in `# x-release-please-start-version` / `# x-release-please-end`; verify that `make -n docker-build` prints `-t ghcr.io/oliverziegert/satisfactory-operator:v0.0.1` with no stray space
- [x] 1.3 Add an `images:` entry to `config/manager/kustomization.yaml` (`newName: ghcr.io/oliverziegert/satisfactory-operator`, `newTag: v0.0.1 # x-release-please-version`) and verify `bin/kustomize build config/default | grep image:` shows the GHCR image
- [x] 1.4 Run `make build-installer` and then `git diff config/manager/kustomization.yaml`; verify the `x-release-please-version` comment survives `kustomize edit set image`. If not, make the target restore the file and re-verify

## 2. release-please

- [x] 2.1 Add `release-please-config.json` (package `"."`, `release-type: go`, `bump-minor-pre-major: true`, `release-as: "0.1.0"`, `changelog-sections` with visible `chore`, `extra-files` for Makefile, kustomization and README) and `.release-please-manifest.json` (`{".": "0.0.0"}`); verify both parse with `jq . <file>`
- [x] 2.2 Add `.github/workflows/release.yml` with the `release-please` (App token) and `publish` jobs from design.md (actions pinned by commit SHA with version comment, `concurrency: release`, minimal permissions); verify with `go run github.com/rhysd/actionlint/cmd/actionlint@latest`
- [x] 2.3 Add the README "Install" section with `kubectl apply -f .../releases/download/v0.0.1/install.yaml` between release-please version markers, keeping the existing `make deploy` path for development; verify the markers enclose exactly the URL line

## 3. Commit conventions

- [x] 3.1 Add `.github/workflows/pr-title.yml` (job name `PR Title`, `pull_request_target`, `amannn/action-semantic-pull-request` pinned by SHA, no checkout); verify with actionlint
- [x] 3.2 Update `.github/renovate.json5`: `semanticCommits: "enabled"`, top-level `semanticCommitType: "chore"`, a rule for `gomod`/`dockerfile` managers → `fix`, then `github.com/onsi/**` → `chore`; verify with `npx --yes -p node@24 -p renovate@latest -- renovate-config-validator .github/renovate.json5`
- [x] 3.3 Update the dependency-updates notes in CLAUDE.md to describe the `fix(deps)`/`chore(deps)` split, and verify the text matches the renovate rules

## 4. Docs

- [x] 4.1 Add a "Releasing" section to CLAUDE.md (conventional PR titles, release PR merge = release, `release-as` for 1.0, App secrets names, how to verify attestations) and verify each command it names exists in the repo or workflow

## 5. Integration (after merge, needs repo admin)

- [x] 5.1 Create a GitHub App (contents + pull-requests read/write), install it on this repo only, and store its Client ID as `RELEASE_APP_CLIENT_ID` (create-github-app-token v3 deprecates `app-id`) and `RELEASE_APP_PRIVATE_KEY` as repo secrets; verify that `gh secret list` shows both
- [x] 5.2 Set `squash_merge_commit_title=PR_TITLE` (`gh api -X PATCH repos/oliverziegert/satisfactory-operator -f squash_merge_commit_title=PR_TITLE -f squash_merge_commit_message=COMMIT_MESSAGES`) and add `PR Title` to the required checks on `main`; verify via `gh api repos/oliverziegert/satisfactory-operator/branches/main/protection`
- [x] 5.3 Verify that the `0.1.0` release PR is opened by the App, runs Lint/Tests/E2E Tests/PR Title, and bumps the Makefile, kustomization and README
- [x] 5.4 Merge the release PR. Verify that tag `v0.1.0`, the GitHub Release with `install.yaml`, and image tags `v0.1.0` + `v0.1` (no `latest`) exist, that `docker buildx imagetools inspect` lists amd64+arm64, and that `gh attestation verify` succeeds
- [x] 5.5 Make the GHCR package public and verify an anonymous `docker pull ghcr.io/oliverziegert/satisfactory-operator:v0.1.0` works
- [ ] 5.6 Remove `release-as` from `release-please-config.json` in a `chore:` PR and verify the next release PR proposes a normal SemVer bump

## Workflow follow-up

- Archive the change after the first release has been verified.
