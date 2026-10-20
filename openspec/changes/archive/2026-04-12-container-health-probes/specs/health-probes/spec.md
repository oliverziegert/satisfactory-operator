## ADDED Requirements

### Requirement: Readiness probe on game server container
The operator SHALL configure an HTTPS readiness probe on the game server container that verifies the HTTPS API is responding.

#### Scenario: Default readiness probe
- **WHEN** a SatisfactoryServer resource is created without `readinessProbeOverride`
- **THEN** the container SHALL have a readiness probe with `httpGet` on path `/api/v1` using `scheme: HTTPS` on the game port, `initialDelaySeconds: 30`, `periodSeconds: 10`, `failureThreshold: 3`

#### Scenario: Custom readiness probe
- **WHEN** a SatisfactoryServer resource specifies `readinessProbeOverride` with custom values
- **THEN** the container SHALL use the override probe definition instead of the default

### Requirement: Liveness probe on game server container
The operator SHALL configure a TCP socket liveness probe on the game server container that verifies the game server process is alive.

#### Scenario: Default liveness probe
- **WHEN** a SatisfactoryServer resource is created without `livenessProbeOverride`
- **THEN** the container SHALL have a liveness probe with `tcpSocket` on the game port, `initialDelaySeconds: 120`, `periodSeconds: 30`, `failureThreshold: 10`

#### Scenario: Custom liveness probe
- **WHEN** a SatisfactoryServer resource specifies `livenessProbeOverride` with custom values
- **THEN** the container SHALL use the override probe definition instead of the default

### Requirement: Probe override spec fields
The `SatisfactoryServerSpec` SHALL include optional `readinessProbeOverride` and `livenessProbeOverride` fields of type `*corev1.Probe` that completely replace the default probes when set.

#### Scenario: Override fields absent
- **WHEN** both override fields are nil
- **THEN** the operator SHALL use the default probe definitions

#### Scenario: Override fields present
- **WHEN** either override field is non-nil
- **THEN** the operator SHALL use the override value as the complete probe definition with no merging of defaults