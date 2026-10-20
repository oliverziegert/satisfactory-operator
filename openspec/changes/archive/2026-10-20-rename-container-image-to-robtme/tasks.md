# Tasks

## 1. Default image

- [x] 1.1 In `api/v1alpha1/satisfactoryserver_types.go`, change the `Image` doc comment and `+kubebuilder:default` to `ghcr.io/robtme/satisfactory-server:v1.9.10`, run `make manifests`, and verify that `config/crd/bases/satisfactory.oliverziegert.github.com_satisfactoryservers.yaml` contains the new default and no `wolveix`
- [x] 1.2 In `internal/controller/satisfactoryserver_helpers.go`, change the `buildPodSpec` fallback image to `ghcr.io/robtme/satisfactory-server:v1.9.10` and update the `buildEnvVars` comment to name `robtme/satisfactory-server`
- [x] 1.3 In `internal/controller/satisfactoryserver_helpers_test.go`, update the default and custom image assertions to the new name (custom: `ghcr.io/robtme/satisfactory-server:latest`), and verify `go test ./internal/controller/ -run 'TestBuildPodSpec|TestBuildStatefulSet'` passes

## 2. Policy allowlist

- [x] 2.1 In `internal/controller/clusterpolicy_helpers_test.go` and `clusterpolicy_controller_test.go`, switch the image strings to `ghcr.io/robtme/satisfactory-server:…`, and in `TestValidateImageAllowlist` add subtests for the two new spec scenarios: `docker.io/robtme/…:v1.9.10` is rejected against an allowlist holding only `robtme/…:v1.9.10`, and `ghcr.io/robtme/…:latest` is rejected against one holding only `:v1.9.10`. Verify with `make test`
- [x] 2.2 In `config/samples/satisfactory_v1alpha1_clusterpolicy.yaml`, set `allowedImageVersions` to `ghcr.io/robtme/`, `robtme/` and `docker.io/robtme/` `satisfactory-server` at `v1.9.10` and `v1.9.9` (6 entries), with a comment saying matching is exact (registry, repository, tag). Verify with `kubectl apply --dry-run=client -f` on the sample

## 3. Samples and docs

- [x] 3.1 In `config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml`, set `image: ghcr.io/robtme/satisfactory-server:v1.9.10`
- [x] 3.2 In `CLAUDE.md`, change the image default line to `ghcr.io/robtme/satisfactory-server` (formerly `wolveix/`)

## 4. Integration check

- [x] 4.1 Run `make build test lint`. Verify everything passes and that `grep -rn wolveix --exclude-dir=archive --exclude-dir=bin .` finds matches only in `openspec/specs/` (updated when the change is archived) and in this change's own files
