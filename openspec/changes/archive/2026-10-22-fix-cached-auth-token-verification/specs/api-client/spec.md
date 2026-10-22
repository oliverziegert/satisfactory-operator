## MODIFIED Requirements

### Requirement: Structured error handling
The client SHALL define an `APIError` type that contains `ErrorCode` (string), `ErrorMessage` (string), and `ErrorData` (map) fields. Non-success HTTP responses with JSON bodies SHALL be parsed into `APIError`. Success (2xx) responses whose JSON body contains a non-empty `errorCode` SHALL also be returned as `APIError`.

#### Scenario: API returns error response
- **WHEN** an API call receives a non-2xx response with a JSON body containing `errorCode`
- **THEN** the method SHALL return an `*APIError` that implements the `error` interface

#### Scenario: API returns error with success status
- **WHEN** an API call receives an HTTP 200 response with a JSON body containing `errorCode` (for example `missing_params`)
- **THEN** the method SHALL return an `*APIError` carrying that `ErrorCode` and `ErrorMessage`

#### Scenario: Network or non-JSON error
- **WHEN** an API call fails due to network error or receives a non-JSON response
- **THEN** the method SHALL return a standard Go error (not `*APIError`)

## REMOVED Requirements

### Requirement: VerifyAuthenticationToken endpoint
**Reason**: The target server build (`ghcr.io/robtme/satisfactory-server:v1.9.10`) rejects `VerifyAuthenticationToken` for valid tokens (HTTP 200 `missing_params` without parameters, `401 token_validation_failed` with them), so the method can't verify a token. The operator no longer calls it (#43).
**Migration**: Verify a token with any authenticated call, for example `QueryServerState`, and treat an error as an invalid token.
