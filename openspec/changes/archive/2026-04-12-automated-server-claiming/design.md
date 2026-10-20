## Context

The Satisfactory Dedicated Server starts in an unclaimed state after first deployment. The API client already supports `PasswordlessLogin` (which grants `InitialAdmin` access on unclaimed servers) and `ClaimServer` (which sets the server name and admin password). The `server-auth-secret-management` change provides the `adminPasswordSecretRef` field and token caching infrastructure that this feature builds on.

Currently, the controller detects unclaimed servers via the probe: when `QueryServerState` returns an auth error, the probe returns `Claimed: false`. After claiming, subsequent probes with the admin token will succeed.

## Goals / Non-Goals

**Goals:**

- Automatically claim unclaimed servers when credentials are configured
- Set the in-game server name from the CRD spec
- Provide an opt-out mechanism for users who want manual claiming
- Emit Events for observability

**Non-Goals:**

- Reclaiming a server that was already claimed (claiming is a one-time operation)
- Setting client password during claiming (can be done separately)
- Changing server name after initial claim (use RenameServer separately)

## Decisions

### 1. Claiming happens after probe, not before

The reconciliation flow is: reconcile resources → authenticate → probe → **claim if unclaimed** → re-probe. Claiming depends on knowing the server is reachable and unclaimed, which the probe determines. After a successful claim, the controller requeuess immediately so the next reconciliation picks up the claimed state.

### 2. PasswordlessLogin used to get InitialAdmin token

On an unclaimed server, `PasswordlessLogin` with `InitialAdmin` privilege succeeds without any credentials. This token is used for the `ClaimServer` call. After claiming, the controller uses `PasswordLogin` (from `server-auth-secret-management`) with the admin password for subsequent access.

### 3. autoClaimDisabled defaults to false

Most users deploying via the operator want fully automated provisioning. The opt-out field `autoClaimDisabled` is only for edge cases where claiming must be done manually (e.g., migrating an existing server).

### 4. serverName defaults to the CR name

If `serverName` is empty, the controller uses `server.Name` (the Kubernetes resource name) as the in-game server name. This provides a sensible default without requiring extra configuration.

## Risks / Trade-offs

**[Risk] Race condition if user claims manually during operator startup** → The `ClaimServer` call will fail with an API error (server already claimed). The controller handles this gracefully: the next probe will see `Claimed: true` and skip the claiming step.

**[Risk] Admin password Secret not yet created when server starts** → The controller sets `Authenticated=False` (from the auth change) and skips claiming. Once the Secret is created, the next reconciliation attempts claiming.

**[Trade-off] No rename support** → Changing `serverName` after initial claim does not rename the server. This is intentional: renaming is a separate API call with different semantics. A future change could watch for `serverName` changes and call `RenameServer`.
