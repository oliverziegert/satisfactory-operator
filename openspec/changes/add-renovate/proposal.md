# Proposal

## Why

Every dependency version in this repo (Go modules, Docker base images, GitHub Actions, Makefile-pinned tools) is bumped by hand, so it falls behind silently. Several of these versions are coupled by the Operator SDK / Kubebuilder scaffold, so naive automated bumps would produce broken or quietly drifting builds. We want automated updates that respect those couplings.

## What Changes

- Add a Renovate configuration (`.github/renovate.json5`) for the hosted Mend Renovate GitHub App.
- Group coupled dependencies so they move together:
  - **kubernetes stack**: `k8s.io/*`, `sigs.k8s.io/controller-runtime`, `CONTROLLER_TOOLS_VERSION`.
  - **go toolchain**: `go` directive in `go.mod` and the `golang` builder image.
  - **golangci-lint**: `GOLANGCI_LINT_VERSION` in the Makefile and `version:` in `lint.yml`.
- Gate scaffold-level upgrades behind Dependency Dashboard approval: `OPERATOR_SDK_VERSION`, opm, kubernetes-stack minor/major, and all majors.
- Pin Docker images (`golang`, `distroless/static`) and GitHub Actions to digests/SHAs.
- Automerge (via PR) low-risk updates: digests, kubernetes-stack patches, GitHub Actions minor/patch, ginkgo/gomega minor/patch, golangci-lint.
- Add custom regex managers for Makefile-pinned versions (`OPERATOR_SDK_VERSION`, `KUSTOMIZE_VERSION`, `CONTROLLER_TOOLS_VERSION`, `GOLANGCI_LINT_VERSION`, opm URL).
- Add a CI check in `test.yml` that fails when generated code (`make manifests generate`) or `go.mod`/`go.sum` (`go mod tidy`) is stale, replacing the current silent `go mod tidy` before tests.

## Capabilities

### New Capabilities
- `dependency-updates`: how the project's dependencies are kept current — which updates are proposed, grouped, gated or automerged, and how CI guards against stale generated code.

### Modified Capabilities
<!-- none -->

## Impact

- New file: `.github/renovate.json5`.
- Modified: `.github/workflows/test.yml` (drift check), `.github/workflows/*.yml` (SHA-pinned actions, on Renovate's onboarding/first run), `Dockerfile` (digest-pinned images, on first run).
- Makefile version variables gain `# renovate:` annotations for the regex managers.
- Contributors (human or Renovate) must commit regenerated CRDs/RBAC/deepcopy and a tidy `go.mod`, or CI fails.
- Prerequisite outside the repo: install the Renovate GitHub App and mark the `Lint` and `Tests` checks as required on `main` before relying on automerge.
