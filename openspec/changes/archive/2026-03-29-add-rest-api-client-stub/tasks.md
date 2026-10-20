## 1. Project Setup

- [x] 1.1 Create `pkg/satisfactory/` directory structure
- [x] 1.2 Vendor the OpenAPI spec file `satisfactory-api-spec.yaml` (v0.2.1) into the project at `pkg/satisfactory/api-spec/satisfactory-api-spec.yaml`

## 2. Types Definition

- [x] 2.1 Create `pkg/satisfactory/types.go` with the `PrivilegeLevel` string type and constants (NotAuthenticated, Client, Administrator, InitialAdmin, APIToken)
- [x] 2.2 Define `APIError` struct with `ErrorCode`, `ErrorMessage`, `ErrorData` fields and implement the `error` interface
- [x] 2.3 Define authentication response types: `HealthCheckResponse`, `PasswordlessLoginResponse`, `PasswordLoginResponse`
- [x] 2.4 Define server state types: `ServerGameState`, `QueryServerStateResponse`, `ClaimServerResponse`, `SetAdminPasswordResponse`
- [x] 2.5 Define settings types: `ServerOptions` (map alias), `GetServerOptionsResponse`, `AdvancedGameSettings`, `GetAdvancedGameSettingsResponse`
- [x] 2.6 Define save game types: `SaveHeader`, `SessionSaveStruct`, `EnumerateSessionsResponse`, `NewGameData`

## 3. Client Core

- [x] 3.1 Create `pkg/satisfactory/client.go` with `Client` struct containing `baseURL`, `httpClient`, and `authToken` fields
- [x] 3.2 Implement `NewClient(baseURL string, tlsConfig *tls.Config) *Client` constructor
- [x] 3.3 Implement `SetAuthToken(token string)` method
- [x] 3.4 Implement internal `doRequest(ctx, functionName, requestBody)` method for standard JSON POST requests with bearer token injection
- [x] 3.5 Implement internal response parsing: success path (2xx) and error path (non-2xx to `*APIError`)

## 4. Authentication Endpoints

- [x] 4.1 Create `pkg/satisfactory/auth.go` with `HealthCheck(ctx, clientCustomData) (*HealthCheckResponse, error)` method
- [x] 4.2 Implement `VerifyAuthenticationToken(ctx) error` method
- [x] 4.3 Implement `PasswordlessLogin(ctx, minimumPrivilegeLevel) (*PasswordlessLoginResponse, error)` method
- [x] 4.4 Implement `PasswordLogin(ctx, minimumPrivilegeLevel, password) (*PasswordLoginResponse, error)` method

## 5. Server Management Endpoints

- [x] 5.1 Create `pkg/satisfactory/server.go` with `QueryServerState(ctx) (*QueryServerStateResponse, error)` method
- [x] 5.2 Implement `ClaimServer(ctx, serverName, adminPassword) (*ClaimServerResponse, error)` method
- [x] 5.3 Implement `RenameServer(ctx, serverName) error` method
- [x] 5.4 Implement `SetClientPassword(ctx, password) error` method
- [x] 5.5 Implement `SetAdminPassword(ctx, password) (*SetAdminPasswordResponse, error)` method
- [x] 5.6 Implement `SetAutoLoadSessionName(ctx, sessionName) error` method
- [x] 5.7 Implement `RunCommand(ctx, command) error` method
- [x] 5.8 Implement `Shutdown(ctx) error` method

## 6. Game Settings Endpoints

- [x] 6.1 Create `pkg/satisfactory/settings.go` with `GetServerOptions(ctx) (*GetServerOptionsResponse, error)` method
- [x] 6.2 Implement `ApplyServerOptions(ctx, options map[string]string) error` method
- [x] 6.3 Implement `GetAdvancedGameSettings(ctx) (*GetAdvancedGameSettingsResponse, error)` method
- [x] 6.4 Implement `ApplyAdvancedGameSettings(ctx, settings map[string]string) error` method

## 7. Save Game Endpoints

- [x] 7.1 Create `pkg/satisfactory/savegame.go` with `CreateNewGame(ctx, newGameData *NewGameData) error` method
- [x] 7.2 Implement `SaveGame(ctx, saveName) error` method
- [x] 7.3 Implement `LoadGame(ctx, saveName, enableAdvancedGameSettings) error` method
- [x] 7.4 Implement `DeleteSaveFile(ctx, saveName) error` method
- [x] 7.5 Implement `DeleteSaveSession(ctx, sessionName) error` method
- [x] 7.6 Implement `EnumerateSessions(ctx) (*EnumerateSessionsResponse, error)` method
- [x] 7.7 Implement `UploadSaveGame(ctx, saveName, loadSaveGame, enableAdvancedGameSettings, saveFileReader io.Reader) error` with multipart/form-data encoding
- [x] 7.8 Implement `DownloadSaveGame(ctx, saveName) (io.ReadCloser, error)` returning streaming binary response

## 8. Verification

- [x] 8.1 Run `go build ./pkg/satisfactory/...` to verify compilation
- [x] 8.2 Run `go vet ./pkg/satisfactory/...` to check for issues
- [x] 8.3 Add unit tests for `APIError` type and `NewClient` constructor in `pkg/satisfactory/client_test.go`