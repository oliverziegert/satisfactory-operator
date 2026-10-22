# Proposal

## Why

The operator never reuses its cached auth token (#43). Every SatisfactoryServer and SaveGame reconcile logs "Cached auth token is invalid, re-authenticating" and logs in again with the admin password. It checks the token with `VerifyAuthenticationToken`. The server build we target (`ghcr.io/robtme/satisfactory-server:v1.9.10`) rejects that call with HTTP 200 `missing_params`, and with `401 token_validation_failed` once the parameters are sent, even for a valid token. With the 60s status poll, that is one extra request per server per minute carrying the admin password, plus noisy logs. `QueryServerState` accepts the same token.

## What Changes

- The controller checks a cached token with an authenticated `QueryServerState` call instead of `VerifyAuthenticationToken`. If the check fails for any reason, it re-authenticates with `PasswordLogin` and updates the cached token, as it does today.
- The "re-authenticating" log line includes the error from the check.
- **BREAKING** (Go API of `pkg/satisfactory`): remove `Client.VerifyAuthenticationToken`. The operator is its only consumer, the module is pre-1.0, and the method doesn't work against the target server build. The vendored API spec YAML (`pkg/satisfactory/api-spec/`) stays unchanged because it is an upstream copy.
- The existing client behavior that an HTTP 2xx response with an `errorCode` body is returned as `*APIError` gets written into the spec and covered by a test. That mechanism hid this bug.

## Capabilities

### New Capabilities

### Modified Capabilities
- `server-auth`: cached tokens are checked with an authenticated server-state query instead of `VerifyAuthenticationToken`, and a valid cached token never triggers `PasswordLogin`.
- `api-client`: remove the `VerifyAuthenticationToken` endpoint requirement; structured error handling also covers 2xx responses that carry an `errorCode`.

## Impact

- Code: `internal/controller/satisfactoryserver_auth.go` (`authenticateClient`), `pkg/satisfactory/auth.go` (remove `VerifyAuthenticationToken`).
- Tests: `internal/controller/satisfactoryserver_auth_test.go`, `internal/controller/savegame_helpers_test.go` (fake game API stub), `pkg/satisfactory/client_test.go`.
- Both callers of `authenticateClient` (the server probe and the SaveGame client resolution) get the fix without changes.
- No CRD, RBAC or dependency change. Condition reasons stay the same.
