# Tasks

## 1. API client

- [x] 1.1 Remove `Client.VerifyAuthenticationToken` from `pkg/satisfactory/auth.go`; verify `go build ./...` fails only at the controller call site that 2.1 replaces
- [x] 1.2 Add a test to `pkg/satisfactory/client_test.go`: an `httptest` server returning HTTP 200 with `{"errorCode":"missing_params","errorMessage":"..."}` makes the call return `*satisfactory.APIError` with `ErrorCode == "missing_params"`; verify with `go test ./pkg/satisfactory/ -run <TestName> -v`

## 2. Controller

- [x] 2.1 In `authenticateClient` (`internal/controller/satisfactoryserver_auth.go`), check the cached token with `c.QueryServerState(ctx)` instead of `VerifyAuthenticationToken`, include the error in the "Cached auth token is invalid, re-authenticating" log line (`"error", err`), and update the flow doc comment; verify `go build ./...` succeeds
- [x] 2.2 Rework `TestAuthenticateClient_CachedTokenValid` in `internal/controller/satisfactoryserver_auth_test.go`: the stub answers `QueryServerState` with 200 + data, and `VerifyAuthenticationToken` (if still requested) with HTTP 200 `missing_params`; assert `Authenticated=True` and that `PasswordLogin` was never called
- [x] 2.3 Rework `TestAuthenticateClient_CachedTokenExpired`: `QueryServerState` returns `401 token_validation_failed`; assert `PasswordLogin` was called, `Authenticated=True`, and the `my-server-auth-token` Secret now holds the new token
- [x] 2.4 Remove the `VerifyAuthenticationToken` cases from the `TestAuthenticateClient_SuccessfulLogin` stub and from the fake game API in `internal/controller/savegame_helpers_test.go`, and make the fake answer `QueryServerState` with 200 + data; verify `make test` passes

## 3. Verification

- [x] 3.1 Run `make lint` and `make test`; both pass
- [x] 3.2 Against a running v1.9.10 server, confirm that after the first login, reconciles no longer log "Cached auth token is invalid, re-authenticating" and `Authenticated` stays `True`

## Workflow follow-up

- PR title: `fix: verify cached auth token with QueryServerState` (closes #43).
- Archive the change after merge.
