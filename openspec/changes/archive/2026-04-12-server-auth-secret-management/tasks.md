## 1. CRD and API Changes

- [x] 1.1 Add `adminPasswordSecretRef` field (`*corev1.LocalObjectReference`) to `SatisfactoryServerSpec` in `api/v1alpha1/satisfactoryserver_types.go`
- [x] 1.2 Add `SatisfactoryServerConditionTypeAuthenticated = "Authenticated"` constant in `api/v1alpha1/satisfactoryserver_types.go`
- [x] 1.3 Run `make manifests generate` to regenerate CRD manifests and deepcopy methods

## 2. RBAC

- [x] 2.1 Add `// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch` marker to `satisfactoryserver_controller.go`
- [x] 2.2 Run `make manifests` and verify the generated ClusterRole includes Secret permissions

## 3. Authentication Logic

- [x] 3.1 Add helper method `getAdminPassword(ctx, server) (string, error)` that reads the referenced Secret and extracts the `password` key
- [x] 3.2 Add helper method `getCachedAuthToken(ctx, server) (string, error)` that reads the `<server-name>-auth-token` Secret and returns the `token` key
- [x] 3.3 Add helper method `cacheAuthToken(ctx, server, token) error` that creates or updates the `<server-name>-auth-token` Secret with an owner reference
- [x] 3.4 Add method `authenticateClient(ctx, server, client) error` that orchestrates: read cached token → verify → re-authenticate if needed → cache new token → set `Authenticated` condition
- [x] 3.5 Integrate `authenticateClient` into the `Reconcile` method between StatefulSet readiness check and game API probe

## 4. Probe Integration

- [x] 4.1 Refactor `probeGameServer` to accept a `*satisfactory.Client` parameter instead of creating a new client internally
- [x] 4.2 Create the client in `Reconcile`, optionally authenticate it, then pass it to `probeGameServer`
- [x] 4.3 Update `setGameConditions` to handle the `Authenticated` condition (set/clear based on auth state)

## 5. Tests

- [x] 5.1 Add unit tests for `getAdminPassword` — Secret exists, Secret missing, key missing
- [x] 5.2 Add unit tests for `getCachedAuthToken` and `cacheAuthToken` — create, update, owner reference
- [x] 5.3 Add unit tests for `authenticateClient` — successful login, failed login, cached token valid, cached token expired
- [x] 5.4 Update existing controller reconciliation tests to cover the authenticated probe path
- [x] 5.5 Run `make test` and verify all tests pass

## 6. Samples and Documentation

- [x] 6.1 Update `config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml` to include an example `adminPasswordSecretRef`
- [x] 6.2 Add a sample Secret manifest alongside the sample CR