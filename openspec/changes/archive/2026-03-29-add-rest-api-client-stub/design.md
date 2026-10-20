## Context

The Satisfactory Dedicated Server exposes an HTTPS API on port 7777 (same as game traffic) for server management. All endpoints use POST to `/api/v1/?function=<FunctionName>` with JSON request/response bodies. Authentication uses bearer tokens (JWT) with privilege levels. The server uses self-signed TLS certificates. An official OpenAPI v3.1 spec is maintained at github.com/satisfactory-oas/spec (v0.2.1, 20 endpoints).

The operator's controllers will need this client to reconcile CRD state against actual server state.

## Goals / Non-Goals

**Goals:**
- Provide a hand-written, idiomatic Go HTTP client package at `pkg/satisfactory/`
- Cover all 20 API endpoints with typed request/response structs matching the OpenAPI spec
- Support bearer token authentication and self-signed TLS certificates
- Provide structured error handling based on the API's error response format
- Vendor the OpenAPI spec YAML in the repo for reference

**Non-Goals:**
- Auto-generating the client from the OpenAPI spec (the spec's unusual URL-based routing `?function=X` makes standard generators produce awkward code)
- Retry logic or circuit breakers (those belong in the controller layer)
- Connection pooling or rate limiting (the operator manages one server per SatisfactoryServer resource)
- Token refresh or session management (controllers handle when to re-authenticate)

## Decisions

### 1. Hand-written client over code generation

The Satisfactory API uses a non-standard routing pattern where all endpoints POST to the same base path with a `?function=` query parameter, and each request body includes a `function` field. Standard OpenAPI Go generators (oapi-codegen, openapi-generator) produce awkward interfaces for this pattern. A hand-written client gives us clean, idiomatic Go methods.

**Alternatives considered:**
- oapi-codegen — generates type-safe code but the function-dispatch pattern produces confusing method signatures
- openapi-generator (Go) — heavy runtime dependency, generates too much boilerplate for this simple API

### 2. Package structure: `pkg/satisfactory/`

Place the client in `pkg/satisfactory/` following the operator-sdk convention of `pkg/` for shared library code. Organize into:
- `client.go` — Client struct, constructor, HTTP plumbing
- `types.go` — All request/response Go types
- `auth.go` — Authentication endpoint methods (HealthCheck, PasswordlessLogin, PasswordLogin, VerifyAuthenticationToken)
- `server.go` — Server management methods (QueryServerState, ClaimServer, RenameServer, SetClientPassword, SetAdminPassword, SetAutoLoadSessionName, RunCommand, Shutdown)
- `settings.go` — Game settings methods (GetServerOptions, ApplyServerOptions, GetAdvancedGameSettings, ApplyAdvancedGameSettings)
- `savegame.go` — Save game methods (CreateNewGame, SaveGame, LoadGame, DeleteSaveFile, DeleteSaveSession, EnumerateSessions, UploadSaveGame, DownloadSaveGame)

### 3. Client constructor accepts base URL and TLS config

The client takes a base URL (e.g., `https://server-ip:7777`) and a `*tls.Config` to allow skipping certificate verification for self-signed certs. Authentication token is set via a method after login.

**Alternatives considered:**
- Passing token in constructor — rejected because the token isn't available until after login
- Functional options pattern — overkill for this simple config; can add later if needed

### 4. Error handling via typed APIError

The API returns errors with `errorCode`, `errorMessage`, and optional `errorData`. We define an `APIError` struct implementing the `error` interface. Non-2xx responses that return JSON are parsed into `APIError`. This allows callers to check error types programmatically.

### 5. Multipart handling for UploadSaveGame

UploadSaveGame uses `multipart/form-data` with a JSON `data` part and a binary `saveGameFile` part. This endpoint gets a dedicated implementation using `mime/multipart` rather than the standard JSON request flow.

### 6. DownloadSaveGame returns an io.ReadCloser

DownloadSaveGame responds with a binary file attachment on success. The method returns an `io.ReadCloser` for the save file data so callers can stream it without buffering the entire file in memory.

## Risks / Trade-offs

- **[API version drift]** The OpenAPI spec (v0.2.1) may lag behind the actual server API → Vendor the spec file and document the version; update when new releases appear
- **[Self-signed TLS]** Skipping cert verification reduces security → Make it configurable; default to verifying, allow opt-out via client config
- **[No generated code]** Hand-written types may drift from the spec → Include the vendored spec as reference; add tests that validate types match expected schema
- **[Blocking HTTP calls]** All client methods are synchronous → Acceptable for controller reconciliation loops which are sequential by design