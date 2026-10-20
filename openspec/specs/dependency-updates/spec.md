# dependency-updates Specification

## Purpose
Keeps the project's dependencies current through automated update proposals while respecting the version couplings imposed by the Operator SDK scaffold.

## Requirements
### Requirement: Automated update proposals
The repository SHALL have automated dependency updates proposed as pull requests for Go modules, Dockerfile images, GitHub Actions and Makefile-pinned tool versions.

#### Scenario: New version of a tracked dependency
- **WHEN** a newer version of a tracked dependency is published
- **THEN** a pull request updating that dependency is opened, unless a rule in this spec gates or groups it

#### Scenario: Makefile-pinned tool
- **WHEN** a newer release of `kustomize`, `controller-tools`, `golangci-lint`, `operator-sdk` or `opm` is published
- **THEN** the corresponding version pinned in the Makefile is recognised as an updatable dependency

### Requirement: Operator SDK scaffold upgrades are gated
Updates to the Operator SDK version and opm SHALL NOT open a pull request until the maintainer approves them on the Dependency Dashboard.

#### Scenario: New Operator SDK release
- **WHEN** a new `operator-sdk` release is published
- **THEN** it is listed on the Dependency Dashboard as awaiting approval and no pull request exists until approved

### Requirement: Kubernetes stack moves as one unit
Updates to `k8s.io/*` modules, `controller-runtime` and `controller-tools` SHALL be proposed together in a single pull request.

#### Scenario: Patch release of the stack
- **WHEN** patch releases are available for stack members
- **THEN** one pull request updates all of them and is automerged once required checks pass

#### Scenario: Minor release of the stack
- **WHEN** a new minor version is available for any stack member
- **THEN** the grouped update awaits Dependency Dashboard approval before a pull request is opened

### Requirement: Go toolchain moves as one unit
The `go` directive in `go.mod` and the Go builder image SHALL be updated together, and minor toolchain updates SHALL NOT be automerged.

#### Scenario: New Go minor release
- **WHEN** a new Go minor version is released
- **THEN** one pull request updates both the `go` directive and the builder image and waits for manual merge

#### Scenario: Go patch release
- **WHEN** only a Go patch release is available
- **THEN** the `go` directive is not changed

### Requirement: Pinned images and actions
Docker images in the Dockerfile SHALL be pinned by digest and GitHub Actions SHALL be pinned by commit SHA, with the human-readable version retained.

#### Scenario: Image digest changes upstream
- **WHEN** the digest behind a pinned image tag changes
- **THEN** a pull request updating the digest is opened and automerged once required checks pass

#### Scenario: Action reference
- **WHEN** a workflow references a GitHub Action
- **THEN** the reference is a commit SHA annotated with its version tag

### Requirement: Low-risk updates automerge
Digest updates, GitHub Actions minor/patch updates, ginkgo/gomega minor/patch updates and golangci-lint updates SHALL be automerged through a pull request after required checks pass.

#### Scenario: Checks pass
- **WHEN** an automerge-eligible pull request passes all required checks
- **THEN** it is merged without manual action and remains visible in pull request history

#### Scenario: Checks fail
- **WHEN** an automerge-eligible pull request fails a required check
- **THEN** it is not merged

### Requirement: Single golangci-lint version
The golangci-lint version used locally and in CI SHALL be updated together in one pull request.

#### Scenario: New golangci-lint release
- **WHEN** a new golangci-lint version is released
- **THEN** one pull request updates both the Makefile and the lint workflow to the same version

### Requirement: Majors require approval
Major version updates of any dependency SHALL await Dependency Dashboard approval before a pull request is opened.

#### Scenario: New major version
- **WHEN** a new major version of any dependency is released
- **THEN** it is listed on the Dependency Dashboard awaiting approval

### Requirement: Go module files stay tidy on updates
Every Go module update pull request SHALL include the result of `go mod tidy`.

#### Scenario: Direct dependency bump
- **WHEN** a pull request updates a Go module
- **THEN** `go.mod` and `go.sum` in that pull request are tidy

### Requirement: CI rejects stale generated files
CI SHALL fail when committed generated code or Go module files differ from what `make manifests generate` and `go mod tidy` produce.

#### Scenario: controller-tools bump without regeneration
- **WHEN** a pull request changes the controller-tools version but not the generated CRDs, RBAC or deepcopy code it would produce
- **THEN** the test workflow fails and reports the differing files

#### Scenario: Up-to-date change
- **WHEN** generated files and `go.mod`/`go.sum` are current
- **THEN** the drift check passes
