## Why

When a new SatisfactoryServer is deployed, the game server starts in an unclaimed state. An admin must manually open a browser, navigate to the server, and claim it before the server is usable. The controller already detects `ServerClaimed=False` but takes no action. The API client already implements `PasswordlessLogin` and `ClaimServer`. Automating this eliminates a manual step and enables fully declarative server provisioning.

**Depends on:** `server-auth-secret-management` (needs `adminPasswordSecretRef` for the admin password used during claiming).

## What Changes

- Add `serverName` field to `SatisfactoryServerSpec` — the in-game display name set during claiming
- Add `autoClaimDisabled` bool field to `SatisfactoryServerSpec` — allows users to opt out of auto-claiming (default: `false`, meaning auto-claim is enabled)
- When `ServerClaimed=False` and `ServerReachable=True` and `adminPasswordSecretRef` is set and `autoClaimDisabled` is not `true`:
  1. Call `PasswordlessLogin` with `PrivilegeLevelInitialAdmin` to get an initial token
  2. Call `ClaimServer(serverName, adminPassword)` using the password from the Secret
  3. Store the returned auth token in the `<server-name>-auth-token` Secret
  4. Set `ServerClaimed=True`
- Emit a Kubernetes Event on successful claim

## Capabilities

### New Capabilities

- `server-claiming`: The controller automatically claims unclaimed game servers using the configured server name and admin password, transitioning them from initial setup to operational state.

### Modified Capabilities

- `server-lifecycle`: New `serverName` and `autoClaimDisabled` spec fields. Reconciliation loop now includes a claiming step.
- `status-conditions`: `ServerClaimed` condition now transitions to `True` via controller action (not just observation).

## Impact

- `api/v1alpha1/satisfactoryserver_types.go` — two new spec fields
- `internal/controller/satisfactoryserver_controller.go` — claiming logic after probe when `ServerClaimed=False`
- `config/crd/bases/` — regenerated CRD manifests
