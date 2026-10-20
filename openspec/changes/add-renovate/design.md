# Design

## Context

The project was scaffolded with Operator SDK v1.42.2 (Kubebuilder `go.kubebuilder.io/v4`). Versions are spread across:

- `go.mod`: `go 1.24.0`, `k8s.io/*` v0.33.0, `controller-runtime` v0.21.0, ginkgo/gomega, `robfig/cron/v3`.
- `Dockerfile`: `golang:1.24`, `gcr.io/distroless/static:nonroot`.
- `.github/workflows/{lint,test,test-e2e}.yml`: `actions/checkout@v4`, `actions/setup-go@v5` (follows `go.mod` via `go-version-file`), `golangci/golangci-lint-action@v8` with `version: v2.14.0`.
- `Makefile`: `OPERATOR_SDK_VERSION`, `KUSTOMIZE_VERSION`, `CONTROLLER_TOOLS_VERSION`, `GOLANGCI_LINT_VERSION`, and opm `v1.55.0` inline in a curl URL.
- `ENVTEST_VERSION` / `ENVTEST_K8S_VERSION` are derived from `go.mod` at make time and need no management.

`test.yml` runs `go mod tidy` and then `make test`, which regenerates manifests and deepcopy code in the workspace. Stale committed files therefore pass CI unnoticed.

Renovate runs as the hosted Mend GitHub App, which cannot run arbitrary post-upgrade commands such as `make manifests generate`. It does support `postUpdateOptions: ["gomodTidy"]`.

## Goals / Non-Goals

**Goals:**
- One config file, readable with comments, encoding the grouping and gating decisions in `specs/dependency-updates/spec.md`.
- Keep the change small: annotate existing Makefile variables instead of restructuring the Makefile.

**Non-Goals:**
- Automating Operator SDK scaffold migrations. They stay a manual, approval-gated step.
- Regenerating code inside Renovate PRs. CI flags it and the maintainer regenerates.
- Managing the kind version in `test-e2e.yml`. It installs `latest` and is left as is.
- Schedules and PR rate limits beyond Renovate defaults.

## Decisions

**Config location `.github/renovate.json5`, extends `config:recommended`.** JSON5 allows comments explaining each regex manager and gate. `config:recommended` already enables the Dependency Dashboard, semantic grouping of monorepos and vulnerability alerts.

**Makefile versions via a single generic regex manager with `# renovate:` annotations.** Each pinned variable gets a preceding comment, e.g.
`# renovate: datasource=github-releases depName=operator-framework/operator-sdk`
`OPERATOR_SDK_VERSION ?= v1.42.2`.
One `customManagers` entry matches `# renovate: datasource=(?<datasource>\S+) depName=(?<depName>\S+)\n\S+ \?= (?<currentValue>\S+)`. The alternative, one hand-written regex per variable, means more config and more breakage risk.
- Datasources: operator-sdk → `github-releases operator-framework/operator-sdk`; opm → `github-releases operator-framework/operator-registry`; kustomize → `go sigs.k8s.io/kustomize/kustomize/v5`; controller-tools → `go sigs.k8s.io/controller-tools`; golangci-lint → `go github.com/golangci/golangci-lint/v2`.
- opm is currently inline in a URL. Extract it to `OPM_VERSION ?= v1.55.0`, used in the URL, so the same manager covers it. That's a one-line refactor, cheaper than a URL-specific regex.

**golangci-lint in `lint.yml` via a second regex manager** matching `version: (?<currentValue>v\S+)` in `.github/workflows/lint.yml`, with the same `depName` as the Makefile entry. Renovate merges identical depNames into one PR, and an explicit `groupName: "golangci-lint"` makes that certain.

**Package rules (order matters, last match wins):**
1. All `major` updates → `dependencyDashboardApproval: true`.
2. `operator-framework/operator-sdk`, `operator-framework/operator-registry` → `dependencyDashboardApproval: true` for every update type.
3. Kubernetes stack: `matchPackageNames: ["k8s.io/**", "sigs.k8s.io/controller-runtime", "sigs.k8s.io/controller-tools"]` → `groupName: "kubernetes stack"`. Patch → `automerge: true`. Minor → `dependencyDashboardApproval: true`. `k8s.io/utils` (pseudo-version, digest-like) is excluded from the group so it doesn't block it.
4. Go toolchain: `matchDepNames: ["go", "golang"]` → `groupName: "go toolchain"`, no automerge. Disable `patch` for the `gomod` manager's `go` directive. Digest updates of `golang` are not part of the group and automerge (rule 5).
5. Automerge set: `matchUpdateTypes: ["digest", "pinDigest"]`; `matchManagers: ["github-actions"]` + minor/patch; `matchPackageNames: ["github.com/onsi/**"]` + minor/patch; `groupName: "golangci-lint"`. All use `automerge: true` with the default `automergeType: "pr"`.

**Pinning presets:** `docker:pinDigests` and `helpers:pinGitHubActionDigests`. Renovate opens the initial pin PRs itself, so `Dockerfile` and workflows are not pinned by hand. The `docker-buildx` sed that injects `--platform=${BUILDPLATFORM}` after `FROM` still works with `image:tag@sha256:...`.

**`postUpdateOptions: ["gomodTidy"]`** so Go PRs pass the new drift check.

**Drift check in `test.yml`.** Replace the `go mod tidy` + `make test` step with:
1. `make manifests generate && go mod tidy`
2. `git diff --exit-code` (fails with the list of stale files)
3. `make test`

This lives in `test.yml` rather than a new workflow, since that job already sets up Go and its check is the one marked required. `test-e2e.yml` keeps its `go mod tidy`. It is not the drift gate.

## Risks / Trade-offs

- [Automerge without required checks merges untested code] → Prerequisite in tasks: make `Lint` and `Tests` required on `main` before installing the Renovate App.
- [Renovate's handling of the `go` directive differs from expectations, e.g. it's skipped or bumped on patch] → Verify on the Dependency Dashboard after onboarding, then adjust the rule.
- [Regex manager silently stops matching after a Makefile edit or SDK re-scaffold] → The Dependency Dashboard lists detected dependencies. Check that all five Makefile tools appear, and re-check after each SDK migration because scaffolds rewrite the Makefile.
- [Digest automerge churn adds noise to history] → Accepted. The user chose reproducible builds over a quiet history.
- [Kubernetes stack patch automerge brings a controller-tools patch that changes generated output] → The drift check fails the PR and blocks automerge, and the maintainer regenerates on the branch.

## Migration Plan

1. Merge the config, Makefile annotations and drift check on `main` (the repo must be on GitHub first).
2. Set required checks on `main`.
3. Install the Renovate App for the repo. Because the config is already on `main`, Renovate skips the onboarding PR and starts with the pin PRs and the Dependency Dashboard issue.
4. Rollback: delete `.github/renovate.json5` or uninstall the App. The drift check is independently useful and stays.
