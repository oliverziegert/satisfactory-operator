## ADDED Requirements

### Requirement: Authenticated condition for SatisfactoryServer
The system SHALL define an `Authenticated` condition type constant for SatisfactoryServer that indicates whether the controller holds a valid API token for the game server.

#### Scenario: Authenticated condition constant exists
- **WHEN** the `api/v1alpha1` package is compiled
- **THEN** a string constant `SatisfactoryServerConditionTypeAuthenticated = "Authenticated"` SHALL exist

#### Scenario: Authenticated is True after successful login
- **WHEN** the controller successfully authenticates via `PasswordLogin` or verifies a cached token
- **THEN** the `Authenticated` condition SHALL be `True` with reason `TokenValid`

#### Scenario: Authenticated is False when Secret not found
- **WHEN** the `adminPasswordSecretRef` references a nonexistent Secret
- **THEN** the `Authenticated` condition SHALL be `False` with reason `SecretNotFound`

#### Scenario: Authenticated is False when login fails
- **WHEN** `PasswordLogin` returns an authentication error
- **THEN** the `Authenticated` condition SHALL be `False` with reason `AuthenticationFailed`

#### Scenario: Authenticated is Unknown when no credentials configured
- **WHEN** `adminPasswordSecretRef` is not set on the SatisfactoryServer resource
- **THEN** the `Authenticated` condition SHALL not be set (absent from conditions array)