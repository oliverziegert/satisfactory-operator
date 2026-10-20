# server-auth Specification

## Purpose
Authenticate the operator with the game server API using an admin password Secret, caching and re-verifying the resulting token.

## Requirements
### Requirement: Admin password Secret reference
The `SatisfactoryServerSpec` SHALL include an optional `adminPasswordSecretRef` field of type `corev1.LocalObjectReference` that references a Secret in the same namespace containing the admin password under key `password`.

#### Scenario: Secret reference configured
- **WHEN** a `SatisfactoryServer` resource is created with `adminPasswordSecretRef.name` set to `"my-server-admin"`
- **THEN** the controller SHALL attempt to read the Secret `my-server-admin` in the same namespace and extract the `password` key

#### Scenario: Secret reference not configured
- **WHEN** a `SatisfactoryServer` resource is created without `adminPasswordSecretRef`
- **THEN** the controller SHALL skip authentication and probe the game server API without a bearer token (preserving current behavior)

### Requirement: Authentication via PasswordLogin
When `adminPasswordSecretRef` is set and the game server is reachable, the controller SHALL authenticate by calling `PasswordLogin` with `PrivilegeLevelAdministrator` and the password from the referenced Secret.

#### Scenario: Successful authentication
- **WHEN** the controller reads the admin password from the Secret and calls `PasswordLogin`
- **THEN** the controller SHALL receive an authentication token, call `SetAuthToken` on the client, and proceed with an authenticated probe

#### Scenario: Authentication failure due to wrong password
- **WHEN** `PasswordLogin` returns an `APIError` indicating invalid credentials
- **THEN** the controller SHALL set `Authenticated=False` with reason `AuthenticationFailed`, emit a Kubernetes Event, and proceed with an unauthenticated probe

#### Scenario: Referenced Secret does not exist
- **WHEN** `adminPasswordSecretRef` references a Secret that does not exist
- **THEN** the controller SHALL set `Authenticated=False` with reason `SecretNotFound`, emit a Kubernetes Event, and proceed with an unauthenticated probe

#### Scenario: Referenced Secret missing password key
- **WHEN** the referenced Secret exists but does not contain a `password` key
- **THEN** the controller SHALL set `Authenticated=False` with reason `SecretKeyMissing` and proceed with an unauthenticated probe

### Requirement: Auth token caching in managed Secret
The controller SHALL cache the authentication token in a Secret named `<server-name>-auth-token` in the same namespace as the `SatisfactoryServer` resource. The token SHALL be stored under key `token`.

#### Scenario: Token cached after successful login
- **WHEN** `PasswordLogin` succeeds and returns an authentication token
- **THEN** the controller SHALL create or update a Secret named `<server-name>-auth-token` with the token value under key `token`

#### Scenario: Managed Secret has owner reference
- **WHEN** the controller creates the `<server-name>-auth-token` Secret
- **THEN** the Secret SHALL have an owner reference pointing to the `SatisfactoryServer` resource so it is garbage collected on deletion

#### Scenario: Cached token reused on subsequent reconciliation
- **WHEN** the `<server-name>-auth-token` Secret exists with a token value
- **THEN** the controller SHALL read the cached token and call `VerifyAuthenticationToken` before using it

### Requirement: Token verification and re-authentication
Before using a cached token, the controller SHALL call `VerifyAuthenticationToken`. If verification fails, the controller SHALL re-authenticate via `PasswordLogin` and update the cached token.

#### Scenario: Cached token is valid
- **WHEN** `VerifyAuthenticationToken` succeeds for the cached token
- **THEN** the controller SHALL use the cached token for the current probe without calling `PasswordLogin`

#### Scenario: Cached token is expired or invalid
- **WHEN** `VerifyAuthenticationToken` returns an error for the cached token
- **THEN** the controller SHALL call `PasswordLogin` to obtain a fresh token, update the `<server-name>-auth-token` Secret, and use the new token

### Requirement: RBAC for Secrets
The SatisfactoryServer controller SHALL have RBAC permissions to get, list, watch, create, update, and patch Secrets.

#### Scenario: RBAC markers generate Secret permissions
- **WHEN** the controller source code is processed by controller-gen
- **THEN** the generated ClusterRole SHALL include rules for `core/secrets` with verbs `get`, `list`, `watch`, `create`, `update`, `patch`
