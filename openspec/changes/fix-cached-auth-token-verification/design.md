# Design

## Context

`authenticateClient` (`internal/controller/satisfactoryserver_auth.go`) is shared by the server probe (`satisfactoryserver_controller.go`) and SaveGame client resolution (`savegame_helpers.go`). Its flow is: cached token → verify → otherwise read the admin password → `PasswordLogin` → cache. Only the verify step is broken. See proposal.md (Why).

## Goals / Non-Goals

**Goals:**
- A valid cached token is reused without `PasswordLogin`.
- The fix stays inside `authenticateClient`, so neither caller changes.

**Non-Goals:**
- Lazy re-login on `401` from real API calls (the probe or SaveGame calls).
- Telling transport errors apart from auth rejections in the check.
- Changing the vendored upstream API spec.

## Decisions

- **Verify with `QueryServerState`.** It is cheap, authenticated, and proven to accept the token on v1.9.10. *Alternatives:* (a) use the cached token without checking it and log in again on a `401` from later calls. That saves one request per reconcile but needs retry logic in both callers and in every SaveGame call. (b) Send `VerifyAuthenticationToken` with its parameters. That doesn't work, because it returns `token_validation_failed` for valid tokens. The probe calls `QueryServerState` again right after the check. That duplicate request is accepted.
- **Any check error triggers re-login (unchanged).** On a transport error the following `PasswordLogin` also fails, which gives `Authenticated=False` with reason `AuthenticationFailed`. `ServerReachable=False` already reports the real cause, and the password never leaves the process because no connection is made. *Alternative:* re-login only on `*APIError`. That needs a new condition state for "check inconclusive" for an edge case that fixes itself on the next reconcile.
- **Delete `Client.VerifyAuthenticationToken`** instead of keeping it with a warning. It has no other caller, and keeping a method we know is broken invites someone to use it again.

## Risks / Trade-offs

- [A future server build might reject `QueryServerState` for some privilege levels] → The operator always logs in as `Administrator`, which has full access. A rejection only brings back today's re-login behavior; it can't lock the operator out.
- [Removing an exported method breaks external importers of `pkg/satisfactory`] → The module is pre-1.0 and no consumers are known. It is flagged **BREAKING** in the proposal.
