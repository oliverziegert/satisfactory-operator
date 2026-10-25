# Tasks

## 1. Spike: plugin viability

- [x] 1.1 Add a pinned `kubebuilder` tool to the Makefile with a `# renovate:` comment, and run `kubebuilder edit --plugins=helm/v2-alpha`. Verify that `dist/chart/` renders with `helm template` and that `PROJECT` lists the plugin.
- [x] 1.2 Add a dummy custom template, re-run the plugin, and check whether it survives. Record the result in `design.md`, and switch to the thin-chart fallback if it doesn't.

## 2. Chart content

- [x] 2.1 Set default values (image tag defaulting to `appVersion`, `crds.install`/`crds.keep` true, `watchNamespaces: []`, `clusterPolicy.enabled: false`) and add the CRD `keep` annotation. Verify with `helm template` output and `helm lint`.
- [x] 2.2 Add the `ClusterPolicy` hook template with `spec` passthrough. Verify that `helm install` with `clusterPolicy.enabled=true` succeeds in one go on a fresh Kind cluster.
- [x] 2.3 Add the namespace-mode RBAC (Role and RoleBinding per namespace, minimal `clusterpolicies` ClusterRole, `--watch-namespaces` arg). Verify with `helm template --set watchNamespaces={games}` and `kubectl auth can-i` checks after install: Secrets allowed in `games`, denied in `default`.
- [x] 2.4 Add `make` targets so that `make helm` (or the plugin's equivalent) regenerates the chart. Extend the `test.yml` drift check to cover `dist/chart`. Verify that the check fails on a hand-edited generated file.

## 3. CI and release

- [x] 3.1 In `lint.yml`, add `helm lint` and `helm template` for the default, `watchNamespaces` and `clusterPolicy` value sets. Verify that the workflow passes on the PR.
- [x] 3.2 Add `Chart.yaml` `version`/`appVersion` with `x-release-please-version` markers to `release-please-config.json` `extra-files`. Verify that the release PR preview bumps them.
- [x] 3.3 In the `release.yml` `publish` job, add `helm package` and `helm push` to `oci://ghcr.io/<owner>/charts`, plus the chart attestation. Verify on the first release with `helm pull` and `gh attestation verify`.

## 4. E2e and docs

- [x] 4.1 Add an ordered e2e spec that installs the chart with `watchNamespaces={games}` and `clusterPolicy.enabled=true`:
  - The manager runs.
  - A server in `games` gets a StatefulSet.
  - A server in `other` gets nothing.
  - The policy defaults are patched onto the `games` server.

  Verify with `make test-e2e`.
- [x] 4.2 Add an e2e migration spec: install via `install.yaml`, create a server, follow the README steps, `helm install`, and check that the StatefulSet and PVC still exist and the server is reconciled. Verify with `make test-e2e`.
- [x] 4.3 Rewrite the README install section to lead with Helm, keep `install.yaml` as cluster-wide plain manifests, and document `watchNamespaces`, the ClusterPolicy hook trade-off and the migration with its deletion warning. Verify that the documented commands match the e2e steps.
- [x] 4.4 Update `CLAUDE.md` (release artifacts, chart drift check, kubebuilder tool). Verify by reading it against the final workflows.

## Workflow follow-up

- Implement after `add-watch-namespaces` is merged.
- Open the PR titled `feat: add Helm chart`.
- Archive the change after merge.
