## Why

The SatisfactoryServer controller probes the game API but never authenticates. It creates a `Client` with no auth token, so `QueryServerState` fails with an auth error on claimed servers — the probe treats this as "reachable but unclaimed." No mechanism exists to store or retrieve admin credentials. This is foundational: all other API-interacting features (auto-claiming, save management) depend on authenticated access.

## What Changes

- Add `adminPasswordSecretRef` field to `SatisfactoryServerSpec` — a `corev1.LocalObjectReference` pointing to a Secret in the same namespace containing key `password` with the admin password
- On reconciliation, if `adminPasswordSecretRef` is set and the server is reachable, read the referenced Secret and call `PasswordLogin` to obtain a bearer token
- Store the auth token in a controller-managed Secret named `<server-name>-auth-token` with key `token` for reuse across reconciliation loops
- Before reusing a cached token, call `VerifyAuthenticationToken` — if it returns an error, re-authenticate via `PasswordLogin`
- Pass the authenticated client to the probe function so `QueryServerState` succeeds on claimed servers
- Add RBAC markers for Secrets (get, list, watch, create, update, patch)
- Add a new condition `Authenticated` to SatisfactoryServer tracking whether the controller holds a valid API token

## Capabilities

### New Capabilities

- `server-auth`: The controller authenticates with the game server API using credentials from a Kubernetes Secret, caches tokens in a managed Secret, and re-authenticates on expiry.

### Modified Capabilities

- `server-lifecycle`: New `adminPasswordSecretRef` spec field. Probe now uses an authenticated client when credentials are available.
- `status-conditions`: New `Authenticated` condition type for SatisfactoryServer.

## Impact

- `api/v1alpha1/satisfactoryserver_types.go` — new spec field, new condition constant
- `internal/controller/satisfactoryserver_controller.go` — auth logic before probe, new condition management, authenticated client passed to `probeGameServer`
- `config/crd/bases/` — regenerated CRD manifests
- `config/rbac/` — new Secret permissions from RBAC markers
