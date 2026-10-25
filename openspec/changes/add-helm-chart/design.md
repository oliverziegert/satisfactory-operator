# Design

## Context

`config/` is generated (`make manifests`) and `install.yaml` is built from `config/default` in `release.yml`. `PROJECT` lists only the operator-sdk plugins (`manifests.sdk`, `scorecard.sdk`), and the Makefile has no `kubebuilder` tool. The e2e suite (`test/e2e/e2e_test.go`) deploys with `make deploy` and only checks that the manager runs and serves metrics. The namespace mode itself comes from `add-watch-namespaces`.

## Goals / Non-Goals

**Goals:**
- A generated chart that stays in sync with `make manifests`, plus a small hand-written part.

**Non-Goals:**
- Charts for game servers or savegames. A separate chart can come later if users ask.
- A separate CRD chart, a `gh-pages` Helm repo, or independent chart versioning.
- A kustomize overlay for namespace mode, or a migration script.
- Helm unit-test plugins.

## Decisions

- **Generate with the Kubebuilder `helm/v2-alpha` plugin into `dist/chart/`.** It builds from the kustomize output and already provides `crd`, `metrics`, `prometheus` and `networkPolicy` toggles. Hand-written charts drift from `config/`. Spike first: confirm that re-running the plugin keeps the custom templates (the ClusterPolicy hook and the namespace-mode RBAC). If it doesn't, fall back to a thin chart that templates the Deployment and values, and copies the CRDs and RBAC from `config/` with a make target.
- **CRDs in templates with `keep`, not `crds/`.** `crds/` never upgrades, which doesn't work for a `v1alpha1` API. The cost is that a CR of a CRD in the same release fails on first install, hence the next decision.
- **ClusterPolicy as a `post-install,post-upgrade` hook**, with `hook-delete-policy: before-hook-creation` so upgrades replace it. Trade-off: hooks aren't release-tracked, so `helm uninstall` leaves the policy behind. That's documented as acceptable, because the CRD is kept as well.
- **Namespace-mode RBAC in templates.** `if watchNamespaces` renders `range` Role/RoleBinding from the same rules as the generated `manager-role`, plus a ClusterRole limited to `clusterpolicies` (get/list/watch/patch/update) and `clusterpolicies/status`/`finalizers`. The manager gets `--watch-namespaces={{ join "," .Values.watchNamespaces }}`. To keep rules from drifting, the Role reuses the generated rule list rather than restating it, if the plugin exposes it. Otherwise a CI `helm template` check asserts that the Role verbs match `config/rbac/role.yaml`.
- **Lockstep OCI release.** `Chart.yaml` gets `x-release-please-version` markers and goes into `extra-files`. The `publish` job runs `helm package` and `helm push` to `oci://ghcr.io/<repo-owner>/charts`, logged in with `GITHUB_TOKEN` (`packages: write` is already granted), and attests the chart digest the same way the image is attested.
- **Migration documented, not scripted.** The steps:
  1. Delete the operator Deployment, RBAC and namespace by explicit kind and name, never the CRDs.
  2. Label and annotate the three CRDs with `app.kubernetes.io/managed-by=Helm`, `meta.helm.sh/release-name` and `meta.helm.sh/release-namespace`.
  3. Run `helm install`.

## Risks / Trade-offs

- [`helm/v2-alpha` is alpha and may change output between Kubebuilder versions] → Pin the kubebuilder version in the Makefile. Renovate updates need dashboard approval, like `OPERATOR_SDK_VERSION`.
- [The chart and `config/` diverge] → The drift check in `test.yml` regenerates `dist/chart` and fails on a diff.
- [A user runs `kubectl delete -f install.yaml` and loses servers] → A warning box at the top of the README migration section, and the migration is covered in e2e.
- [Values or template-only chart fixes force an operator release] → Accepted below 1.0 (lockstep decision).
- [Namespaces in `watchNamespaces` don't exist] → `helm install` fails on the RoleBinding. Documented. The chart doesn't create them.

## Migration Plan

Existing `install.yaml` users follow the README migration. Rollback: `helm uninstall` (the CRDs and CRs are kept), then `kubectl apply -f install.yaml`.

## Open Questions

- Whether Artifact Hub listing (`artifacthub-repo.yml` pushed to the OCI repo) happens in this change or right after. It doesn't affect the chart itself.
