## Why

The operator's controllers need to communicate with the Satisfactory Dedicated Server HTTPS API to manage server lifecycle, configuration, and save games. A typed Go client generated from the official OpenAPI v3.1 spec (satisfactory-oas/spec v0.2.1) provides compile-time safety, consistent error handling, and a clean abstraction layer that all controllers can share.

## What Changes

- Add the official OpenAPI spec file (`satisfactory-api-spec.yaml` v0.2.1) to the project as the source of truth for client generation
- Add a Go HTTP client package (`pkg/satisfactory`) that covers all 20 API endpoints:
  - **Authentication**: HealthCheck, VerifyAuthenticationToken, PasswordlessLogin, PasswordLogin
  - **Server Management**: QueryServerState, ClaimServer, RenameServer, SetClientPassword, SetAdminPassword, SetAutoLoadSessionName, RunCommand, Shutdown
  - **Game Settings**: GetServerOptions, ApplyServerOptions, GetAdvancedGameSettings, ApplyAdvancedGameSettings
  - **Save Game Operations**: CreateNewGame, SaveGame, LoadGame, DeleteSaveFile, DeleteSaveSession, EnumerateSessions, UploadSaveGame, DownloadSaveGame
- Define Go types for all request/response schemas from the OpenAPI spec
- Implement bearer token authentication handling with support for all privilege levels (NotAuthenticated, Client, Administrator, InitialAdmin, APIToken)
- Handle HTTPS with self-signed certificate support (the game server uses self-signed certs)

## Capabilities

### New Capabilities
- `api-client`: Typed Go HTTP client for the Satisfactory Dedicated Server HTTPS API, covering all endpoints with request/response types, authentication, and error handling

### Modified Capabilities
<!-- No existing capabilities to modify -->

## Impact

- **New package**: `pkg/satisfactory/` containing the API client, types, and error definitions
- **New dependency**: The OpenAPI spec YAML file vendored in the project for reference
- **TLS**: Client must support connecting to servers with self-signed certificates
- **Controllers**: All three controllers (SatisfactoryServer, SaveGame, ClusterPolicy) will use this client to interact with game server instances