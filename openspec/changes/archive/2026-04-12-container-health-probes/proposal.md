## Why

The StatefulSet created by the operator has no liveness or readiness probes. Kubernetes has no way to detect a hung game server process or determine when the server is ready to accept player connections. The Satisfactory Dedicated Server exposes a `HealthCheck` endpoint on its HTTPS API (same port as game traffic, default 7777) that returns `"healthy"` when the tick rate is above 10 ticks/sec and `"slow"` otherwise. This endpoint requires no authentication.

**No dependencies** — uses existing container-level HTTP endpoint.

## What Changes

- Add an HTTPS readiness probe to the container spec targeting the HealthCheck API endpoint
- Add an HTTPS liveness probe with a longer failure threshold to tolerate the server's lengthy initial startup (first install downloads ~8GB, subsequent starts take 1-3 minutes)
- Configure probes with `scheme: HTTPS` (Kubernetes skips TLS cert verification for probes)
- Add optional `readinessProbeOverride` and `livenessProbeOverride` fields to `SatisfactoryServerSpec` so users can customize thresholds
- Set sensible defaults: readiness probe with 10s period and 3 failure threshold, liveness probe with 30s period and 10 failure threshold (5 minutes before restart)

## Capabilities

### New Capabilities

- `health-probes`: Kubernetes liveness and readiness probes configured on the game server container using the HealthCheck HTTPS API endpoint.

### Modified Capabilities

- `container-image-config`: The container spec now includes probe definitions in addition to ports, env vars, volumes, and resources.

## Impact

- `api/v1alpha1/satisfactoryserver_types.go` — optional probe override fields
- `internal/controller/satisfactoryserver_helpers.go` — probe definitions added to `buildPodSpec`
- `config/crd/bases/` — regenerated CRD manifests