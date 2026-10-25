# Proposal

## Why

The only install path is a fixed `install.yaml`. Users can't configure the image, resources, scheduling or metrics without forking YAML. GitOps tools (Argo CD, Flux) and Artifact Hub expect a Helm chart. A chart is also the natural place for the namespace-scoped RBAC that `add-watch-namespaces` makes possible.

## What Changes

- New Helm chart for the operator, generated from the kustomize config by the Kubebuilder `helm/v2-alpha` plugin into `dist/chart/`.
- The chart deploys:
  - The CRDs, as templates with `helm.sh/resource-policy: keep` behind `crds.install`/`crds.keep`, so they upgrade with the chart and survive uninstall.
  - RBAC and the manager.
  - Optionally, one `ClusterPolicy`, as a post-install/post-upgrade hook with `spec` passthrough.
- The chart does **not** create `SatisfactoryServer`, `SaveGame` or admin-password Secrets. Those stay user-owned CRs.
- `watchNamespaces: []`: empty renders today's ClusterRole. A list renders a Role and RoleBinding per namespace, plus a minimal ClusterRole for `clusterpolicies`, and passes `--watch-namespaces`.
- Values: image, pull secrets, resources, scheduling, security contexts, metrics, ServiceMonitor, NetworkPolicy, CRDs, `clusterPolicy`.
- Releases publish the chart as OCI to `oci://ghcr.io/oliverziegert/charts/satisfactory-operator`, versioned in lockstep with the operator. `Chart.yaml` joins the release-please version references.
- CI: `helm lint` and `helm template` for both modes. The stale-generated-files check also covers `dist/chart`. A new e2e spec installs the chart in namespace mode and checks the migration from `install.yaml`.
- README: Helm-first install, `install.yaml` kept as "plain manifests, cluster-wide", and a documented migration from `install.yaml` that never deletes the CRDs.

## Capabilities

### New Capabilities
- `helm-chart`: installing and configuring the operator via Helm, including CRD lifecycle, the optional ClusterPolicy, namespace-scoped RBAC and migration from `install.yaml`.

### Modified Capabilities
- `release-automation`: releases also publish the OCI chart, and `Chart.yaml` becomes a tracked version reference.
- `cluster-policy`: the RBAC requirement covers namespace mode (cluster-level rules limited to ClusterPolicy).

## Impact

- New `dist/chart/`, plus one hand-written hook template for `ClusterPolicy`.
- `PROJECT` gains the helm plugin entry. Needs the `kubebuilder` CLI as a Makefile tool, with a `# renovate:` comment like the other tools.
- `.github/workflows/release.yml` (package and push the chart), `lint.yml`, `test.yml` (drift), `test-e2e.yml` (Helm in Kind).
- `release-please-config.json` `extra-files`, `README.md`, `test/e2e/`.
- Depends on `add-watch-namespaces`.
