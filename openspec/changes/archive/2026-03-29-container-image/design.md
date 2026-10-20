## Context

The operator deploys Satisfactory Dedicated Server instances using the community container image `wolveix/satisfactory-server` (Docker Hub: `wolveix/satisfactory-server`). This image wraps SteamCMD + the Satisfactory Dedicated Server binary, handles auto-updates on startup, and exposes configuration via environment variables. The operator's SatisfactoryServer controller must translate CRD spec fields into a Pod specification that correctly configures this image.

Key image characteristics:
- **Ports**: 7777/TCP (server query + HTTPS API), 7777/UDP (game traffic), 8888/TCP (reliable messaging)
- **Volume**: `/config` — stores gamefiles (~8GB), saves, backups, logs
- **Process**: Runs as user PUID/PGID (default 1000:1000)
- **Updates**: Downloads/updates game files on every container start unless SKIPUPDATE=true

## Goals / Non-Goals

**Goals:**
- Define a clear, complete mapping from SatisfactoryServer CRD spec fields to `wolveix/satisfactory-server` environment variables
- Establish sensible defaults matching the image's own defaults
- Define the Pod spec template the controller will generate (ports, volumes, env, resources)
- Support image version pinning via tag

**Non-Goals:**
- Building or maintaining our own container image
- Supporting alternative Satisfactory server images (can be added later)
- Init containers or sidecar containers (future enhancement)
- SSL/TLS certificate management for the game server (future enhancement)

## Decisions

### 1. Default image: `wolveix/satisfactory-server:v1.9.10`

Use `wolveix/satisfactory-server` as the default image. The CRD spec includes an `image` field to allow overriding the full image reference (registry, name, tag).

**Alternatives considered:**
- Official dedicated server image — none exists as a container image
- Custom-built image — unnecessary maintenance burden when the community image is well-maintained (1M+ Docker pulls)

### 2. Environment variable mapping over config files

The `wolveix/satisfactory-server` image uses environment variables exclusively for configuration. The operator maps CRD spec fields directly to env vars on the container. No ConfigMap or mounted config files are needed.

**Alternatives considered:**
- ConfigMap mounted as file — the image doesn't read config files, only env vars
- Passing CLI args — the image's entrypoint doesn't support arbitrary server args

### 3. Single PVC mounted at /config

The image stores everything under `/config` (gamefiles, saves, backups, logs). A single PVC per server instance is sufficient. The CRD spec allows configuring storage size and storage class.

**Alternatives considered:**
- Separate PVCs for saves vs gamefiles — adds complexity without clear benefit; the image expects a single mount
- EmptyDir for gamefiles + PVC for saves — gamefiles are ~8GB and would re-download on every pod restart

### 4. SKIPUPDATE=true by default in operator-managed pods

In a Kubernetes context, game updates should be managed deliberately (e.g., by updating the image tag or CRD spec) rather than happening automatically on pod restart. Setting SKIPUPDATE=true prevents unexpected 8GB+ downloads and unpredictable startup times.

**Alternatives considered:**
- Default SKIPUPDATE=false (image default) — risky in Kubernetes where pods restart frequently
- Making it non-configurable — too opinionated; some users may want auto-updates

### 5. Resource defaults: 4Gi memory request, 8Gi memory limit

Following the image documentation and official Satisfactory wiki recommendations. CPU is left without a default limit as the game server's CPU usage varies significantly with factory complexity.

## Risks / Trade-offs

- **[Large PVC]** Gamefiles alone are ~8GB, plus saves → Default storage request should be at least 15Gi
- **[Startup time]** First boot downloads ~8GB of game files even with SKIPUPDATE=true (initial install) → Document that first pod creation takes significant time
- **[PUID/PGID]** Kubernetes security contexts may conflict with the image's user ID expectations → Map CRD security context to PUID/PGID env vars
