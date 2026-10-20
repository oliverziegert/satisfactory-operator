# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kubernetes operator for managing Satisfactory Dedicated Game Servers. Built with Kubebuilder (go.kubebuilder.io/v4) and Operator SDK, using controller-runtime v0.21.0 on Go 1.27.

API group: `satisfactory.oliverziegert.github.com`, version: `v1alpha1`

## Common Commands

```bash
# Build
make build                    # Build manager binary (includes manifests, generate, fmt, vet)
go build -o bin/manager cmd/main.go  # Build without prerequisites

# Test
make test                     # Unit tests with envtest (excludes e2e)
# Single test:
KUBEBUILDER_ASSETS="$(bin/setup-envtest use -p path)" go test ./internal/controller/ -run TestBuildPodSpec -v  # plain unit tests by name
KUBEBUILDER_ASSETS="$(bin/setup-envtest use -p path)" go test ./internal/controller/ -run TestControllers -ginkgo.focus "SatisfactoryServer"  # envtest suite

# E2e (requires Kind)
make test-e2e                 # Creates Kind cluster, runs e2e, tears down

# Lint
make lint                     # golangci-lint v2
make lint-fix                 # Auto-fix

# Code generation (run after changing *_types.go)
make manifests                # Regenerate CRDs, RBAC, webhook manifests
make generate                 # Regenerate deepcopy methods

# Run locally
make run                      # Run controller against ~/.kube/config cluster
make install                  # Install CRDs into cluster
make uninstall                # Remove CRDs

# Deploy
make deploy IMG=<registry>/satisfactory-operator:tag
make undeploy
```

## Architecture

### Custom Resources (api/v1alpha1/)

- **SatisfactoryServer** (namespaced) - Primary CR. Declares a game server instance with config (image, ports, player limits, tick rate, storage, resources). The controller creates a StatefulSet + headless Service + external (LoadBalancer) Service.
- **SaveGame** (namespaced) - References a SatisfactoryServer via `serverRef`. One-shot or cron-scheduled saves through the game API, with retention by count and age.
- **ClusterPolicy** (cluster-scoped) - Cluster-wide defaults and constraints for SatisfactoryServer resources (resource limits, storage class, server config, allowed images, max servers per namespace).

### Controllers (internal/controller/)

**SatisfactoryServerReconciler** is the most complex controller:
1. Manages finalizer for PVC cleanup on deletion (based on `retainPVCOnDelete`)
2. Reconciles StatefulSet, headless Service, external Service (helpers in `satisfactoryserver_helpers.go`)
3. Authenticates with the admin password from `adminPasswordSecretRef` and caches the token in a managed Secret `<name>-auth-token` (`satisfactoryserver_auth.go`)
4. Probes the game server HTTPS API (port 7777 by default) for reachability, claim status, world load state, version and player count
5. Auto-claims an unclaimed server unless `autoClaimDisabled` (`satisfactoryserver_claim.go`)
6. Sets 7 condition types: Ready, Progressing, Degraded, ServerReachable, ServerClaimed, WorldLoaded, Authenticated
7. Derives Phase (Pending/Running/Updating/Error) from conditions

**SaveGameReconciler** resolves an authenticated client from the referenced server, triggers saves as `<saveName>_<UTC timestamp>`, enforces retention and requeues for the next cron tick (helpers in `savegame_helpers.go`). It watches SatisfactoryServer to react to server changes. Conditions: Ready, BackupScheduled. Phases: Pending/Active/Completed/Failed.

**ClusterPolicyReconciler** uses the oldest ClusterPolicy (others get `Ready=False`, reason `MultiplePoliciesDetected`). It patches zero-valued SatisfactoryServer fields with the policy defaults and reports image-allowlist and per-namespace violations as Warning Events and in the `Enforced` condition. Violations don't block servers (helpers in `clusterpolicy_helpers.go`).

### Game Server API Client (pkg/satisfactory/)

Typed HTTP client for the Satisfactory Dedicated Server HTTPS API (spec v0.2.1). Key files:
- `client.go` - HTTP client with `doRequest`/`parseResponse` pattern, bearer token auth
- `types.go` - Request/response types (ServerGameState, SaveHeader, etc.)
- `server.go` - Server management calls (QueryServerState, ClaimServer, etc.)
- `auth.go` - Authentication calls (PasswordLogin, PasswordlessLogin)
- `savegame.go` - Save game calls (EnumerateSessions, SaveGame, LoadGame, etc.)
- `settings.go` - Server settings calls (GetServerOptions, GetAdvancedGameSettings)

All API calls use `POST /api/v1/?function=<name>` with JSON bodies. Responses wrap data in `{"data": {...}}`.

### Conditions Helper (internal/conditions/)

Generic `Set`/`Remove`/`IsTrue` functions with per-CRD typed wrappers (e.g., `SetSatisfactoryServer`, `IsTrueSaveGame`). All conditions stamp `ObservedGeneration`.

### Key Design Decisions

- StatefulSet uses exactly 1 replica (game server is single-instance)
- The game server uses a self-signed TLS cert, so the probe uses `InsecureSkipVerify`
- Container image defaults to `ghcr.io/robtme/satisfactory-server:v1.9.10` (community image, formerly `wolveix/`, amd64 only)
- The `/config` volume is backed by a PVC via StatefulSet volumeClaimTemplate
- `SkipUpdate` defaults to `true` in K8s to avoid unexpected 8GB+ downloads on pod restart

## Testing

Tests use Ginkgo/Gomega with envtest (controller-runtime's test framework) for the `TestControllers` suite, plus plain `testing` unit tests for helpers. Test files are colocated with controllers in `internal/controller/`. The `pkg/satisfactory/client_test.go` tests the API client against an `httptest.Server`.

## Dependency Updates

Renovate (hosted Mend GitHub App) is configured in `.github/renovate.json5`. The Dependency Dashboard issue lists everything it tracks.

- **Needs dashboard approval** before a PR opens: all majors, `OPERATOR_SDK_VERSION` and `OPM_VERSION` (scaffold upgrades), and kubernetes stack minors (`k8s.io/*`, controller-runtime, controller-tools).
- **Commit types**: PR titles are Conventional Commits. `gomod` and `dockerfile` updates (Go modules, golang builder and distroless base images) are `fix(deps)` and trigger a release. Everything else is `chore(deps)`; it is listed in the changelog and can open the release PR, but nothing ships until that PR is merged: GitHub Actions, Makefile tools and the test-only `github.com/onsi/**` modules.
- **Not automerged**: the go toolchain group (`go` directive + `golang` builder image). Go patch releases don't bump the `go` directive.
- **Automerged** after checks pass: digests, kubernetes stack patches, GitHub Actions minor/patch, ginkgo/gomega minor/patch, golangci-lint (Makefile + `lint.yml` in one PR).
- **Red drift check** in `test.yml` ("Check for stale generated files"): Renovate can't run codegen. Check out the branch, then:
  ```bash
  make manifests generate && go mod tidy
  ```
  commit and push.
- The `# renovate: datasource=... depName=...` comments above the Makefile tool versions drive the regex manager. Operator SDK re-scaffolds rewrite the Makefile, so restore those comments afterwards and check that all five tools still appear on the dashboard.
- Validate config changes with `npx --yes -p node@24 -p renovate@latest -- renovate-config-validator .github/renovate.json5` (Renovate requires Node 24).

## Releasing

release-please (`.github/workflows/release.yml`, `release-please-config.json`, `.release-please-manifest.json`) turns Conventional Commits on `main` into releases.

- **PR titles** must be Conventional Commits (required `PR Title` check, `.github/workflows/pr-title.yml`). Squash merges use the PR title as the commit title. Below 1.0, `feat` and breaking changes bump the minor version and `fix` bumps the patch version.
- **Release PR**: release-please keeps one open PR with the next version and the CHANGELOG. It bumps the version between the `x-release-please` markers in `Makefile`, `config/manager/kustomization.yaml` and `README.md`, so never edit those versions by hand. **Merging it is the release**: the same workflow tags `vX.Y.Z`, creates the GitHub Release, pushes `ghcr.io/oliverziegert/satisfactory-operator:vX.Y.Z` and `:vX.Y` (amd64+arm64, no `latest`), and attaches `install.yaml`.
- **Forcing a version** (e.g. 1.0): set `"release-as": "1.0.0"` on the `"."` package in `release-please-config.json` in a `chore:` PR, then remove it after the release.
- **Secrets**: `RELEASE_APP_CLIENT_ID` and `RELEASE_APP_PRIVATE_KEY` belong to a GitHub App installed on this repo only (contents + pull-requests read/write). The release-please job needs the App token so the release PR triggers the required checks; `GITHUB_TOKEN` wouldn't.
- **Local edits**: `make deploy`/`build-installer`/`bundle` run `kustomize edit set image` and then restore `config/manager/kustomization.yaml`, because kustomize drops the version marker comment.
- **Verify a release**:
  ```bash
  gh attestation verify oci://ghcr.io/oliverziegert/satisfactory-operator:vX.Y.Z -R oliverziegert/satisfactory-operator
  docker buildx imagetools inspect ghcr.io/oliverziegert/satisfactory-operator:vX.Y.Z
  ```
