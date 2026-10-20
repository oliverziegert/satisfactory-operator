## 1. CRD Changes

- [x] 1.1 Add `readinessProbeOverride` (*corev1.Probe, optional) to `SatisfactoryServerSpec` in `api/v1alpha1/satisfactoryserver_types.go`
- [x] 1.2 Add `livenessProbeOverride` (*corev1.Probe, optional) to `SatisfactoryServerSpec`
- [x] 1.3 Run `make manifests generate` to regenerate CRD manifests and deepcopy methods

## 2. Probe Builders

- [x] 2.1 Add helper `buildReadinessProbe(spec) *corev1.Probe` in `satisfactoryserver_helpers.go` that returns the override if set, or the default HTTPS GET probe on `/api/v1` with `initialDelaySeconds: 30`, `periodSeconds: 10`, `failureThreshold: 3`
- [x] 2.2 Add helper `buildLivenessProbe(spec) *corev1.Probe` that returns the override if set, or the default TCP socket probe on the game port with `initialDelaySeconds: 120`, `periodSeconds: 30`, `failureThreshold: 10`
- [x] 2.3 Integrate probes into `buildPodSpec` — set `ReadinessProbe` and `LivenessProbe` on the container

## 3. Tests

- [x] 3.1 Add unit tests for `buildReadinessProbe` — default probe, custom override
- [x] 3.2 Add unit tests for `buildLivenessProbe` — default probe, custom override
- [x] 3.3 Update `buildPodSpec` tests to verify probes are present on the container
- [x] 3.4 Update `buildStatefulSet` tests to verify probes propagate through to the StatefulSet pod template
- [x] 3.5 Run `make test` and verify all tests pass

## 4. Samples

- [x] 4.1 Update `config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml` to document the default probe behavior in a comment
