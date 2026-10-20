## Context

The Satisfactory Dedicated Server HTTPS API exposes a `HealthCheck` endpoint at `POST /api/v1/?function=HealthCheck` that returns `{"health": "healthy"}` or `{"health": "slow"}`. This endpoint does not require authentication. The server listens on the game port (default 7777) using a self-signed TLS certificate. Kubernetes HTTP probes with `scheme: HTTPS` skip certificate verification, making them compatible with self-signed certs.

Currently, `buildPodSpec` in `satisfactoryserver_helpers.go` sets image, env vars, ports, volume mounts, and resources but no probes.

## Goals / Non-Goals

**Goals:**

- Detect hung game server processes via liveness probe
- Signal readiness for player connections via readiness probe
- Tolerate long startup times (first install, map loading)
- Allow users to override probe thresholds

**Non-Goals:**

- Startup probes (Kubernetes 1.20+) — could be added later, but liveness with high failure threshold achieves the same effect
- Game-level health checks beyond HealthCheck endpoint (e.g., checking if a world is loaded)
- Probe-based auto-scaling

## Decisions

### 1. HTTP POST probe using httpGet with a path workaround

Kubernetes probes only support `httpGet` (GET requests), but the HealthCheck endpoint expects POST. However, the Satisfactory server also responds to GET requests on the API endpoint (returning a generic status). As a pragmatic alternative, we use a `tcpSocket` probe on the game port for liveness (process is alive and listening) and an `httpGet` probe with `scheme: HTTPS` on `/api/v1` for readiness (HTTPS API is responding).

**Alternative considered:** Exec probe with `curl -X POST`. Rejected because the container image may not include curl, and exec probes are more expensive than socket/HTTP probes.

**Alternative considered:** Custom health endpoint sidecar. Rejected as overkill for this use case.

### 2. Readiness probe: HTTPS GET on /api/v1

The API base path returns a response when the server is up and the API layer is initialized. This is sufficient for readiness — it confirms the HTTPS API is accepting connections. Defaults: `initialDelaySeconds: 30`, `periodSeconds: 10`, `failureThreshold: 3`.

### 3. Liveness probe: TCP socket on game port

A TCP socket check on the game port confirms the process is alive and listening. This is more reliable than HTTP for liveness because it doesn't fail when the server is temporarily slow during map loading. Defaults: `initialDelaySeconds: 120`, `periodSeconds: 30`, `failureThreshold: 10` (5 minutes total before pod restart).

### 4. Override fields are full corev1.Probe pointers

`readinessProbeOverride` and `livenessProbeOverride` are `*corev1.Probe` fields. When non-nil, they completely replace the default probe (no merging). This gives users full control without complex merge logic.

## Risks / Trade-offs

**[Risk] Readiness probe fails during map loading** → The 30s initial delay and 3-failure threshold gives 60s before the pod is marked unready. For very large saves this may not be enough. Users can increase `initialDelaySeconds` or `failureThreshold` via the override field.

**[Risk] Liveness probe restarts pod during long first install** → First install downloads ~8GB via Steam. The 120s initial delay + 300s failure window (10 * 30s) = 7 minutes total. If the download takes longer, the pod restarts. The `skipUpdate: true` default mitigates this because most users won't download on startup. Users with `skipUpdate: false` should increase the liveness threshold via override.

**[Trade-off] No startup probe** → Startup probes would be cleaner for the long first-start case, but they require Kubernetes 1.20+. The high liveness failure threshold achieves a similar effect. A future change can add startup probes when the minimum K8s version is bumped.