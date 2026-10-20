## Why

The operator needs to know which container image to deploy for Satisfactory Dedicated Server instances and how to configure it. Rather than building a custom image, the operator will use the well-maintained community image `wolveix/satisfactory-server` which handles SteamCMD installation, auto-updates, save management, and process supervision. The operator's CRD types and controller logic need to map CRD spec fields to the image's environment variables, ports, and volume mounts.

## What Changes

- Document and codify the `wolveix/satisfactory-server` container image as the standard game server image for the operator
- Define the mapping between `SatisfactoryServer` CRD spec fields and the image's environment variables (MAXPLAYERS, SERVERGAMEPORT, SERVERMESSAGINGPORT, AUTOSAVENUM, MAXTICKRATE, STEAMBETA, SKIPUPDATE, etc.)
- Define the required container ports: 7777/TCP (server query + HTTPS API), 7777/UDP (game traffic), 8888/TCP (reliable messaging)
- Define the required volume mount: `/config` (contains gamefiles, saves, backups, logs)
- Define the default resource requirements: 4GB memory reservation, 8GB memory limit

## Capabilities

### New Capabilities
- `container-image-config`: Defines the container image specification, environment variable mappings, port definitions, volume mounts, and resource defaults that the operator uses when creating Pods for SatisfactoryServer resources

### Modified Capabilities
<!-- No existing capabilities to modify -->

## Impact

- **CRD types**: `SatisfactoryServer` spec fields must align with the image's environment variables
- **Controller logic**: The SatisfactoryServer controller must translate CRD spec into Pod spec with correct env vars, ports, and volumes
- **Image versioning**: The operator must support pinning to specific image tags (e.g., `wolveix/satisfactory-server:latest`, `wolveix/satisfactory-server:v1.8.7`)
- **Storage**: Each server instance requires a PersistentVolumeClaim mounted at `/config`