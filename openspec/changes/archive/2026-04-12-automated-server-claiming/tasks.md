## 1. CRD Changes

- [x] 1.1 Add `serverName` field (string, optional) to `SatisfactoryServerSpec` in `api/v1alpha1/satisfactoryserver_types.go`
- [x] 1.2 Add `autoClaimDisabled` field (*bool, optional, default false) to `SatisfactoryServerSpec`
- [x] 1.3 Run `make manifests generate` to regenerate CRD manifests and deepcopy methods

## 2. Claiming Logic

- [x] 2.1 Add helper method `claimServer(ctx, server, baseURL, adminPassword) error` that calls `PasswordlessLogin` with `InitialAdmin`, then `ClaimServer` with the server name and admin password, and caches the returned token
- [x] 2.2 Add helper `getServerName(server) string` that returns `server.Spec.ServerName` or falls back to `server.Name`
- [x] 2.3 Integrate claiming into the `Reconcile` method: after probe, if `ServerClaimed=False` and `ServerReachable=True` and credentials are configured and `autoClaimDisabled` is not true, call `claimServer`
- [x] 2.4 After successful claim, emit a Kubernetes Event with reason `ServerClaimed` and requeue immediately

## 3. Condition Updates

- [x] 3.1 When claiming succeeds, ensure the next probe picks up `ServerClaimed=True` (via requeue)
- [x] 3.2 When claiming fails due to server already claimed, log and continue normally

## 4. Tests

- [x] 4.1 Add unit tests for `claimServer` — successful claim, server already claimed, PasswordlessLogin failure
- [x] 4.2 Add unit tests for claiming integration in Reconcile — auto-claim enabled, auto-claim disabled, no credentials
- [x] 4.3 Run `make test` and verify all tests pass

## 5. Samples

- [x] 5.1 Update `config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml` to include `serverName` example
