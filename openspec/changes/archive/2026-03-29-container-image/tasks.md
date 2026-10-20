## 1. CRD Type Updates

- [x] 1.1 Add container image field to SatisfactoryServer spec: `Image` (string, default `wolveix/satisfactory-server:v1.9.10`)
- [x] 1.2 Add server config fields to SatisfactoryServer spec: `MaxPlayers` (int, default 4), `AutosaveNum` (int, default 5), `MaxTickRate` (int, default 30), `MaxObjects` (int, default 2162688), `Timeout` (int, default 30)
- [x] 1.3 Add boolean config fields to SatisfactoryServer spec: `ServerStreaming` (default true), `DisableSeasonalEvents` (default false), `Debug` (default false), `Log` (default false)
- [x] 1.4 Add network fields to SatisfactoryServer spec: `GamePort` (int, default 7777), `MessagingPort` (int, default 8888), `Multihome` (string, default `::`)
- [x] 1.5 Add update control fields to SatisfactoryServer spec: `SkipUpdate` (bool, default true), `SteamBeta` (bool, default false), `SteamBetaPassword` (string, optional)
- [x] 1.6 Add user identity fields to SatisfactoryServer spec: `RunAsUser` (int64, default 1000), `RunAsGroup` (int64, default 1000)
- [x] 1.7 Add storage fields to SatisfactoryServer spec: `Storage.Size` (resource.Quantity, default 15Gi), `Storage.StorageClassName` (string, optional)
- [x] 1.8 Add resource fields to SatisfactoryServer spec: `Resources` (corev1.ResourceRequirements, default 4Gi request / 8Gi limit for memory)

## 2. Environment Variable Mapping

- [x] 2.1 Create helper function `buildEnvVars(spec) []corev1.EnvVar` that maps all CRD spec fields to their corresponding environment variables
- [x] 2.2 Map server config fields: MAXPLAYERS, AUTOSAVENUM, MAXTICKRATE, MAXOBJECTS, TIMEOUT
- [x] 2.3 Map boolean config fields: SERVERSTREAMING, DISABLESEASONALEVENTS, DEBUG, LOG
- [x] 2.4 Map network fields: SERVERGAMEPORT, SERVERMESSAGINGPORT, MULTIHOME
- [x] 2.5 Map update control fields: SKIPUPDATE, STEAMBETA, STEAMBETAKEY
- [x] 2.6 Map user identity fields: PUID, PGID

## 3. Pod Spec Generation

- [x] 3.1 Create helper function `buildContainerPorts(spec) []corev1.ContainerPort` that generates port definitions (gamePort/TCP, gamePort/UDP, messagingPort/TCP)
- [x] 3.2 Create helper function `buildVolumeMount() corev1.VolumeMount` for the /config mount
- [x] 3.3 Create helper function `buildPVCSpec(spec) corev1.PersistentVolumeClaimSpec` for storage configuration
- [x] 3.4 Create helper function `buildPodSpec(spec) corev1.PodSpec` that assembles the complete Pod specification with container, env vars, ports, volumes, and resources

## 4. Verification

- [x] 4.1 Run `make generate` to regenerate DeepCopy methods after type changes
- [x] 4.2 Run `make manifests` to regenerate CRD manifests with new fields
- [x] 4.3 Run `make build` to verify compilation
- [x] 4.4 Add unit tests for `buildEnvVars` verifying correct mapping of all environment variables
- [x] 4.5 Add unit tests for `buildContainerPorts` verifying port definitions with default and custom values
- [x] 4.6 Add unit tests for `buildPodSpec` verifying complete Pod spec generation with defaults