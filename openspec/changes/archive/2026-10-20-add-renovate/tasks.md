# Tasks

## 1. CI drift check

- [x] 1.1 In `.github/workflows/test.yml`, replace the `go mod tidy` + `make test` step with: `make manifests generate && go mod tidy`, then `git diff --exit-code`, then `make test`; verify locally by touching a field comment in `api/v1alpha1/*_types.go`, running the same commands and seeing `git diff --exit-code` fail, then reverting
- [x] 1.2 Verify the current tree is drift-free: `make manifests generate && go mod tidy && git diff --exit-code` exits 0 (commit any regenerated files first if not)

## 2. Makefile annotations

- [x] 2.1 Extract the opm version from the curl URL into `OPM_VERSION ?= v1.55.0` and use `$(OPM_VERSION)` in the URL; verify `rm -f bin/opm && make opm` still downloads v1.55.0
- [x] 2.2 Add `# renovate: datasource=... depName=...` comments directly above `OPERATOR_SDK_VERSION`, `OPM_VERSION`, `KUSTOMIZE_VERSION`, `CONTROLLER_TOOLS_VERSION` and `GOLANGCI_LINT_VERSION` (datasources per design.md); verify `make help` and `make build` still work

## 3. Renovate config

- [x] 3.1 Create `.github/renovate.json5` extending `config:recommended`, `docker:pinDigests`, `helpers:pinGitHubActionDigests`, with `postUpdateOptions: ["gomodTidy"]`
- [x] 3.2 Add the Makefile `customManagers` regex entry and the `lint.yml` golangci-lint regex entry
- [x] 3.3 Add the package rules from design.md: majors gated, operator-sdk/opm gated, kubernetes stack group (patch automerge, minor gated, `k8s.io/utils` excluded), go toolchain group (no automerge, no `go` directive patch), automerge set (digests, actions minor/patch, onsi minor/patch, golangci-lint group)
- [x] 3.4 Validate with `npx --yes --package renovate -- renovate-config-validator .github/renovate.json5`; verify it reports the config as valid
- [x] 3.5 Dry-run extraction with `LOG_LEVEL=debug npx --yes --package renovate -- renovate --platform=local --dry-run=extract` and verify all five Makefile tools, the `lint.yml` golangci-lint version, the `go` directive and both Dockerfile images are detected

## 4. Documentation

- [x] 4.1 Add a "Dependency updates" section to `CLAUDE.md` covering: Renovate config location, which updates need dashboard approval, that a red drift check means "run `make manifests generate`, commit, push", and that the Makefile `# renovate:` comments must survive Operator SDK re-scaffolds; verify the section renders and the commands in it run

## Workflow follow-up

- Push to GitHub, then mark the `Lint` and `Tests` checks as required on `main` (manual, repo settings).
- Install the Mend Renovate GitHub App for the repository.
- On the Dependency Dashboard issue, confirm every dependency from 3.5 is listed and the go directive behaves as specced. Adjust the rules if not.
- Archive the change once the first automerged PR and the first gated approval have worked end to end.
