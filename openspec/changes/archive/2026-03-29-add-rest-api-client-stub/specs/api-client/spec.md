## ADDED Requirements

### Requirement: Client constructor
The client package SHALL provide a `NewClient` function that accepts a base URL (string) and a `*tls.Config`, and returns a configured `*Client` instance ready to make API calls.

#### Scenario: Create client with default TLS
- **WHEN** `NewClient("https://server:7777", nil)` is called
- **THEN** a `*Client` SHALL be returned that uses default TLS settings (certificate verification enabled)

#### Scenario: Create client with insecure TLS
- **WHEN** `NewClient` is called with a `*tls.Config` where `InsecureSkipVerify` is `true`
- **THEN** the client SHALL skip TLS certificate verification for all requests

### Requirement: Bearer token authentication
The client SHALL support setting a bearer token for authenticated API calls. The token SHALL be included as an `Authorization: Bearer <token>` header on all subsequent requests.

#### Scenario: Set authentication token
- **WHEN** `client.SetAuthToken(token)` is called with a valid JWT string
- **THEN** all subsequent API calls SHALL include the `Authorization: Bearer <token>` header

#### Scenario: Unauthenticated endpoints
- **WHEN** HealthCheck, PasswordlessLogin, or PasswordLogin is called without a token set
- **THEN** the request SHALL be sent without an `Authorization` header

### Requirement: Structured error handling
The client SHALL define an `APIError` type that contains `ErrorCode` (string), `ErrorMessage` (string), and `ErrorData` (map) fields. Non-success HTTP responses with JSON bodies SHALL be parsed into `APIError`.

#### Scenario: API returns error response
- **WHEN** an API call receives a non-2xx response with a JSON body containing `errorCode`
- **THEN** the method SHALL return an `*APIError` that implements the `error` interface

#### Scenario: Network or non-JSON error
- **WHEN** an API call fails due to network error or receives a non-JSON response
- **THEN** the method SHALL return a standard Go error (not `*APIError`)

### Requirement: HealthCheck endpoint
The client SHALL provide a `HealthCheck(ctx, clientCustomData)` method that calls the HealthCheck API function and returns the server health status ("healthy" or "slow") and optional server custom data.

#### Scenario: Healthy server
- **WHEN** `HealthCheck` is called and the server is running normally
- **THEN** the response SHALL contain `Health: "healthy"` and `ServerCustomData` fields

### Requirement: VerifyAuthenticationToken endpoint
The client SHALL provide a `VerifyAuthenticationToken(ctx)` method that verifies the current bearer token is still valid.

#### Scenario: Valid token
- **WHEN** `VerifyAuthenticationToken` is called with a valid token set
- **THEN** the method SHALL return `nil` error (HTTP 204)

#### Scenario: Invalid token
- **WHEN** `VerifyAuthenticationToken` is called with an expired or invalid token
- **THEN** the method SHALL return an `*APIError` with HTTP 401

### Requirement: PasswordlessLogin endpoint
The client SHALL provide a `PasswordlessLogin(ctx, minimumPrivilegeLevel)` method that attempts passwordless authentication and returns an authentication token on success.

#### Scenario: Unclaimed server login
- **WHEN** `PasswordlessLogin` is called with `InitialAdmin` privilege on an unclaimed server
- **THEN** the response SHALL contain a valid `AuthenticationToken` string

### Requirement: PasswordLogin endpoint
The client SHALL provide a `PasswordLogin(ctx, minimumPrivilegeLevel, password)` method that authenticates with a password and returns an authentication token.

#### Scenario: Successful admin login
- **WHEN** `PasswordLogin` is called with `Administrator` privilege and the correct admin password
- **THEN** the response SHALL contain a valid `AuthenticationToken` string

### Requirement: QueryServerState endpoint
The client SHALL provide a `QueryServerState(ctx)` method that returns the current server game state including session name, connected players, player limit, tech tier, game phase, running status, game duration, pause state, tick rate, and auto-load session name.

#### Scenario: Query running server
- **WHEN** `QueryServerState` is called on a server with a loaded session
- **THEN** the response SHALL contain a `ServerGameState` struct with all required fields populated

### Requirement: ClaimServer endpoint
The client SHALL provide a `ClaimServer(ctx, serverName, adminPassword)` method that claims an unclaimed server and returns a new authentication token.

#### Scenario: Claim unclaimed server
- **WHEN** `ClaimServer` is called with InitialAdmin privileges on an unclaimed server
- **THEN** the response SHALL contain a new `AuthenticationToken` for admin access

### Requirement: RenameServer endpoint
The client SHALL provide a `RenameServer(ctx, serverName)` method that renames the server. Requires Administrator privileges.

#### Scenario: Successful rename
- **WHEN** `RenameServer` is called with a valid name and admin token
- **THEN** the method SHALL return `nil` error (HTTP 204)

### Requirement: SetClientPassword endpoint
The client SHALL provide a `SetClientPassword(ctx, password)` method that sets or removes the client protection password.

#### Scenario: Set client password
- **WHEN** `SetClientPassword` is called with a non-empty password
- **THEN** the method SHALL return `nil` error (HTTP 204) and all existing client tokens SHALL be invalidated server-side

### Requirement: SetAdminPassword endpoint
The client SHALL provide a `SetAdminPassword(ctx, password)` method that updates the admin password and returns a new authentication token.

#### Scenario: Update admin password
- **WHEN** `SetAdminPassword` is called with a new password
- **THEN** the response SHALL contain a new `AuthenticationToken` since the previous token is now invalidated

### Requirement: SetAutoLoadSessionName endpoint
The client SHALL provide a `SetAutoLoadSessionName(ctx, sessionName)` method that configures which session loads on server startup.

#### Scenario: Set auto-load session
- **WHEN** `SetAutoLoadSessionName` is called with a valid session name
- **THEN** the method SHALL return `nil` error (HTTP 204)

### Requirement: RunCommand endpoint
The client SHALL provide a `RunCommand(ctx, command)` method that executes a console command on the server. Requires Administrator privileges.

#### Scenario: Run console command
- **WHEN** `RunCommand` is called with a valid command string
- **THEN** the method SHALL return `nil` error (HTTP 204)

### Requirement: Shutdown endpoint
The client SHALL provide a `Shutdown(ctx)` method that shuts down the dedicated server.

#### Scenario: Shutdown server
- **WHEN** `Shutdown` is called with admin privileges
- **THEN** the method SHALL return `nil` error (HTTP 204) and the server SHALL begin shutdown

### Requirement: GetServerOptions endpoint
The client SHALL provide a `GetServerOptions(ctx)` method that returns current and pending server options as string key-value maps.

#### Scenario: Retrieve server options
- **WHEN** `GetServerOptions` is called
- **THEN** the response SHALL contain `ServerOptions` and `PendingServerOptions` maps

### Requirement: ApplyServerOptions endpoint
The client SHALL provide a `ApplyServerOptions(ctx, options)` method that applies server option changes. Requires Administrator privileges.

#### Scenario: Apply server options
- **WHEN** `ApplyServerOptions` is called with a map of option key-value pairs
- **THEN** the method SHALL return `nil` error (HTTP 204)

### Requirement: GetAdvancedGameSettings endpoint
The client SHALL provide a `GetAdvancedGameSettings(ctx)` method that returns the current advanced game settings as a string key-value map.

#### Scenario: Retrieve advanced settings
- **WHEN** `GetAdvancedGameSettings` is called
- **THEN** the response SHALL contain an `AppliedAdvancedGameSettings` map

### Requirement: ApplyAdvancedGameSettings endpoint
The client SHALL provide a `ApplyAdvancedGameSettings(ctx, settings)` method that applies advanced game setting changes.

#### Scenario: Apply advanced settings
- **WHEN** `ApplyAdvancedGameSettings` is called with a settings map
- **THEN** the method SHALL return `nil` error (HTTP 204) and advanced settings SHALL be auto-enabled if not already

### Requirement: CreateNewGame endpoint
The client SHALL provide a `CreateNewGame(ctx, newGameData)` method that creates and loads a new game session.

#### Scenario: Create new game
- **WHEN** `CreateNewGame` is called with session name, map name, starting location, and skip onboarding flag
- **THEN** the method SHALL return `nil` error (HTTP 202) and the server SHALL begin loading the new session

### Requirement: SaveGame endpoint
The client SHALL provide a `SaveGame(ctx, saveName)` method that saves the current session to a named save file.

#### Scenario: Save current game
- **WHEN** `SaveGame` is called with a save name
- **THEN** the method SHALL return `nil` error (HTTP 204)

### Requirement: LoadGame endpoint
The client SHALL provide a `LoadGame(ctx, saveName, enableAdvancedGameSettings)` method that loads a save game file.

#### Scenario: Load save game
- **WHEN** `LoadGame` is called with a valid save name
- **THEN** the method SHALL return `nil` error (HTTP 204) and the server SHALL begin loading the save

### Requirement: DeleteSaveFile endpoint
The client SHALL provide a `DeleteSaveFile(ctx, saveName)` method that deletes a single save game file.

#### Scenario: Delete save file
- **WHEN** `DeleteSaveFile` is called with a valid save name
- **THEN** the method SHALL return `nil` error (HTTP 204)

### Requirement: DeleteSaveSession endpoint
The client SHALL provide a `DeleteSaveSession(ctx, sessionName)` method that deletes all save files belonging to a session.

#### Scenario: Delete save session
- **WHEN** `DeleteSaveSession` is called with a valid session name
- **THEN** the method SHALL return `nil` error (HTTP 204)

### Requirement: EnumerateSessions endpoint
The client SHALL provide a `EnumerateSessions(ctx)` method that returns all save sessions and their save files, including save headers with metadata (version, build, name, session, duration, timestamps, mod status).

#### Scenario: List sessions with saves
- **WHEN** `EnumerateSessions` is called on a server with save files
- **THEN** the response SHALL contain a `Sessions` array of `SessionSaveStruct` with `SessionName` and `SaveHeaders`, plus a `CurrentSessionIndex`

### Requirement: UploadSaveGame endpoint
The client SHALL provide a `UploadSaveGame(ctx, saveName, loadSaveGame, enableAdvancedGameSettings, saveFileReader)` method that uploads a save game file via multipart/form-data.

#### Scenario: Upload and load save
- **WHEN** `UploadSaveGame` is called with `loadSaveGame: true` and a valid save file reader
- **THEN** the method SHALL return `nil` error (HTTP 204) and the server SHALL load the uploaded save

### Requirement: DownloadSaveGame endpoint
The client SHALL provide a `DownloadSaveGame(ctx, saveName)` method that downloads a save game file, returning an `io.ReadCloser` for streaming the binary data.

#### Scenario: Download save file
- **WHEN** `DownloadSaveGame` is called with a valid save name
- **THEN** the method SHALL return an `io.ReadCloser` containing the save file binary data

### Requirement: Request format compliance
All API requests (except UploadSaveGame) SHALL be sent as HTTP POST to `<baseURL>/api/v1/?function=<FunctionName>` with `Content-Type: application/json`. The JSON body SHALL include a `function` field matching the endpoint name, and a `data` field when the endpoint requires input parameters.

#### Scenario: Standard JSON request
- **WHEN** any non-upload API method is called
- **THEN** the HTTP request SHALL be POST with `Content-Type: application/json` and the body SHALL contain the `function` field

#### Scenario: Upload multipart request
- **WHEN** `UploadSaveGame` is called
- **THEN** the HTTP request SHALL be POST with `Content-Type: multipart/form-data` containing a `data` JSON part and a `saveGameFile` binary part

### Requirement: Go types for all API schemas
The client package SHALL define Go struct types for all request/response schemas defined in the OpenAPI spec, including: `HealthCheckResponse`, `PasswordlessLoginResponse`, `PasswordLoginResponse`, `QueryServerStateResponse` (with nested `ServerGameState`), `ClaimServerResponse`, `SetAdminPasswordResponse`, `GetServerOptionsResponse`, `GetAdvancedGameSettingsResponse`, `EnumerateSessionsResponse` (with nested `SessionSaveStruct` and `SaveHeader`), `NewGameData`, `AdvancedGameSettings`, `ServerOptions`, `PrivilegeLevel`, and `APIError`.

#### Scenario: Type coverage
- **WHEN** the `pkg/satisfactory/types.go` file is compiled
- **THEN** it SHALL define exported Go types for every request and response schema in the OpenAPI v3.1 spec

### Requirement: PrivilegeLevel type
The client SHALL define a `PrivilegeLevel` string type with constants for all privilege levels: `NotAuthenticated`, `Client`, `Administrator`, `InitialAdmin`, `APIToken`.

#### Scenario: Privilege level constants
- **WHEN** privilege level constants are used
- **THEN** they SHALL match the enum values defined in the OpenAPI spec exactly