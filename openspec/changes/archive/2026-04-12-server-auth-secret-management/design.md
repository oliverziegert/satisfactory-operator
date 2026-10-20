## Context

The SatisfactoryServer controller currently creates an unauthenticated `pkg/satisfactory.Client` to probe the game server API. On claimed servers, `QueryServerState` returns an auth error, which the probe interprets as "reachable but unclaimed" — producing incorrect conditions. The API client already implements `PasswordLogin`, `VerifyAuthenticationToken`, and `SetAuthToken`, but the controller never uses them.

The Satisfactory Dedicated Server API uses bearer token authentication. Tokens are obtained via `PasswordLogin` (with the admin password) and remain valid until the server restarts or the password changes. The controller needs a way to receive the admin password and cache the resulting token.

## Goals / Non-Goals

**Goals:**

- Enable the controller to authenticate with the game server API on claimed servers
- Store admin credentials via standard Kubernetes Secret references
- Cache auth tokens across reconciliation loops to avoid re-authenticating every 5 seconds
- Correctly distinguish "unclaimed server" from "claimed but no credentials configured"
- Add an `Authenticated` condition so users can observe auth state via `kubectl`

**Non-Goals:**

- Automated server claiming (separate proposal — depends on this one)
- Secret rotation or external secret operator integration (future work)
- Client-level authentication (only admin-level is needed for the operator)
- Webhook-based Secret validation

## Decisions

### 1. Admin password passed via Secret reference, not inline

The `adminPasswordSecretRef` field references a Secret by name (same namespace). The controller reads key `password` from the Secret data.

**Alternative considered:** Inline password field in the CRD spec. Rejected because CRD specs are stored in etcd and visible via `kubectl get -o yaml`, making password exposure trivial. Secrets support RBAC-controlled access and can be managed by external secret operators.

### 2. Auth token cached in a controller-managed Secret

After a successful `PasswordLogin`, the controller creates/updates a Secret named `<server-name>-auth-token` with the bearer token. On subsequent reconciliations, the controller reads this Secret first, calls `VerifyAuthenticationToken`, and only re-authenticates if verification fails.

**Alternative considered:** In-memory token cache on the reconciler struct. Rejected because controller-runtime reconcilers should be stateless — pod restarts or leader election changes would lose the cache. A Secret survives restarts and works correctly with multiple replicas (though only the leader reconciles).

**Alternative considered:** Store the token in the SatisfactoryServer status. Rejected because status fields are visible to anyone with read access to the CR and tokens are security-sensitive.

### 3. Auth happens between StatefulSet readiness check and game API probe

The reconciliation order becomes:
1. Reconcile StatefulSet, Services (unchanged)
2. Check StatefulSet readiness (unchanged)
3. **New:** If `adminPasswordSecretRef` is set, obtain an authenticated client
4. Probe game server with the (possibly authenticated) client
5. Set conditions, update status (unchanged)

If authentication fails, the controller sets `Authenticated=False` and proceeds with an unauthenticated probe (preserving current behavior for unclaimed servers).

### 4. Unauthenticated probe still supported

When `adminPasswordSecretRef` is not set, the controller behaves exactly as it does today — unauthenticated probe with `ServerClaimed=False` inference from auth errors. This preserves backward compatibility and supports the pre-claiming workflow.

### 5. Owner references on managed auth token Secret

The `<server-name>-auth-token` Secret gets an owner reference pointing to the SatisfactoryServer resource. When the server is deleted, garbage collection cleans up the token Secret automatically. No finalizer needed for this.

## Risks / Trade-offs

**[Risk] Token cached in Secret is stale after server restart** → The controller calls `VerifyAuthenticationToken` before using a cached token. If verification fails, it re-authenticates. The 5-second requeue on unreachable servers means recovery is fast.

**[Risk] Referenced admin password Secret doesn't exist** → The controller sets `Authenticated=False` with reason `SecretNotFound` and proceeds with unauthenticated probe. An Event is emitted to make the misconfiguration visible.

**[Risk] Admin password in Secret is wrong** → `PasswordLogin` returns an `APIError`. The controller sets `Authenticated=False` with reason `AuthenticationFailed` and proceeds unauthenticated. The condition message includes the error code (but not the password).

**[Trade-off] One extra Secret per server** → Acceptable. The auth token Secret is small (~200 bytes) and garbage collected on server deletion.
