/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package satisfactory provides a typed Go HTTP client for the Satisfactory
// Dedicated Server HTTPS API (spec v0.2.1).
// See api-spec/satisfactory-api-spec.yaml for the OpenAPI v3.1 source.
package satisfactory

import "fmt"

// PrivilegeLevel represents the privilege level of an authentication token.
type PrivilegeLevel string

const (
	// PrivilegeLevelNotAuthenticated means the client is not authenticated.
	PrivilegeLevelNotAuthenticated PrivilegeLevel = "NotAuthenticated"
	// PrivilegeLevelClient means the client has player-level privileges.
	PrivilegeLevelClient PrivilegeLevel = "Client"
	// PrivilegeLevelAdministrator means the client has admin privileges.
	PrivilegeLevelAdministrator PrivilegeLevel = "Administrator"
	// PrivilegeLevelInitialAdmin means the client can claim an unclaimed server.
	PrivilegeLevelInitialAdmin PrivilegeLevel = "InitialAdmin"
	// PrivilegeLevelAPIToken means the client is a third-party application.
	PrivilegeLevelAPIToken PrivilegeLevel = "APIToken"
)

// APIError is the structured error returned by the Satisfactory Dedicated Server API.
// Non-2xx responses with a JSON body are parsed into this type.
type APIError struct {
	// ErrorCode is a machine-friendly code indicating the type of error.
	ErrorCode string `json:"errorCode"`
	// ErrorMessage is an optional human-friendly description of the error.
	ErrorMessage string `json:"errorMessage,omitempty"`
	// ErrorData contains optional additional information about the error.
	ErrorData map[string]any `json:"errorData,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.ErrorMessage != "" {
		return fmt.Sprintf("satisfactory API error %s: %s", e.ErrorCode, e.ErrorMessage)
	}
	return fmt.Sprintf("satisfactory API error %s", e.ErrorCode)
}

// ServerOptions is a map of server option keys to their stringified values.
// Example keys: "FG.DSAutoPause", "FG.AutosaveInterval".
type ServerOptions map[string]string

// AdvancedGameSettings holds applied advanced game settings.
type AdvancedGameSettings struct {
	// AppliedAdvancedGameSettings maps setting names to their stringified values.
	// Example keys: "FG.GameRules.NoPower", "FG.PlayerRules.GodMode".
	AppliedAdvancedGameSettings map[string]string `json:"appliedAdvancedGameSettings"`
}

// HealthCheckResponse is the response from the HealthCheck endpoint.
type HealthCheckResponse struct {
	// Health is "healthy" when tick rate is above 10 ticks/sec, "slow" otherwise.
	Health string `json:"health"`
	// ServerCustomData is custom data passed from the server to the client.
	ServerCustomData string `json:"serverCustomData"`
}

// PasswordlessLoginResponse is the response from PasswordlessLogin.
type PasswordlessLoginResponse struct {
	// AuthenticationToken is the token to use for subsequent authenticated requests.
	AuthenticationToken string `json:"authenticationToken"`
}

// PasswordLoginResponse is the response from PasswordLogin.
type PasswordLoginResponse struct {
	// AuthenticationToken is the token to use for subsequent authenticated requests.
	AuthenticationToken string `json:"authenticationToken"`
}

// ServerGameState holds the current game state of the dedicated server.
type ServerGameState struct {
	// ActiveSessionName is the name of the currently loaded game session.
	ActiveSessionName string `json:"activeSessionName"`
	// NumConnectedPlayers is the number of players currently connected.
	NumConnectedPlayers int `json:"numConnectedPlayers"`
	// PlayerLimit is the maximum number of players allowed.
	PlayerLimit int `json:"playerLimit"`
	// TechTier is the maximum tech tier of all unlocked schematics.
	TechTier int `json:"techTier"`
	// ActiveSchematic is the schematic currently set as active milestone.
	ActiveSchematic string `json:"activeSchematic"`
	// GamePhase is the current game phase (None if no game is running).
	GamePhase string `json:"gamePhase"`
	// IsGameRunning is true if a save is currently loaded.
	IsGameRunning bool `json:"isGameRunning"`
	// TotalGameDuration is the total time the current save has been loaded, in seconds.
	TotalGameDuration int `json:"totalGameDuration"`
	// IsGamePaused is true if the game is paused.
	IsGamePaused bool `json:"isGamePaused"`
	// AverageTickRate is the average tick rate in ticks per second.
	AverageTickRate float64 `json:"averageTickRate"`
	// AutoLoadSessionName is the name of the session that loads on server startup.
	AutoLoadSessionName string `json:"autoLoadSessionName"`
}

// QueryServerStateResponse is the response from QueryServerState.
type QueryServerStateResponse struct {
	// ServerGameState holds the current game state.
	ServerGameState ServerGameState `json:"serverGameState"`
}

// ClaimServerResponse is the response from ClaimServer.
type ClaimServerResponse struct {
	// AuthenticationToken is the new admin token to use after claiming.
	AuthenticationToken string `json:"authenticationToken"`
}

// SetAdminPasswordResponse is the response from SetAdminPassword.
type SetAdminPasswordResponse struct {
	// AuthenticationToken is the new admin token (the previous one is invalidated).
	AuthenticationToken string `json:"authenticationToken"`
}

// GetServerOptionsResponse is the response from GetServerOptions.
type GetServerOptionsResponse struct {
	// ServerOptions contains the currently applied server options.
	ServerOptions ServerOptions `json:"serverOptions"`
	// PendingServerOptions contains options that will apply after session/server restart.
	PendingServerOptions ServerOptions `json:"pendingServerOptions"`
}

// GetAdvancedGameSettingsResponse is the response from GetAdvancedGameSettings.
type GetAdvancedGameSettingsResponse struct {
	AdvancedGameSettings
}

// SaveHeader contains metadata about a save game file.
type SaveHeader struct {
	// SaveVersion is the version of the save game format.
	SaveVersion int `json:"saveVersion"`
	// BuildVersion is the changelist of the game that saved this file.
	BuildVersion int `json:"buildVersion"`
	// SaveName is the name of the save game file in the filesystem.
	SaveName string `json:"saveName"`
	// SaveLocationInfo is the location info (undocumented).
	SaveLocationInfo string `json:"saveLocationInfo,omitempty"`
	// MapName is the path to the map package this save is based on.
	MapName string `json:"mapName"`
	// MapOptions contains additional map URL options.
	MapOptions string `json:"mapOptions"`
	// SessionName is the name of the session this save belongs to.
	SessionName string `json:"sessionName"`
	// PlayDurationSeconds is the total play duration in seconds.
	PlayDurationSeconds int `json:"playDurationSeconds"`
	// SaveDateTime is the date and time the save was created.
	SaveDateTime string `json:"saveDateTime"`
	// IsModdedSave is true if this save was created with mods.
	IsModdedSave bool `json:"isModdedSave"`
	// IsEditedSave is true if this save has been modified by third-party tools.
	IsEditedSave bool `json:"isEditedSave"`
	// IsCreativeModeEnabled is true if Advanced Game Settings are enabled.
	IsCreativeModeEnabled bool `json:"isCreativeModeEnabled"`
}

// SessionSaveStruct represents a save session with all its save files.
type SessionSaveStruct struct {
	// SessionName is the name of the save session.
	SessionName string `json:"sessionName"`
	// SaveHeaders contains all save files belonging to this session.
	SaveHeaders []SaveHeader `json:"saveHeaders"`
}

// EnumerateSessionsResponse is the response from EnumerateSessions.
type EnumerateSessionsResponse struct {
	// Sessions is the list of all available save sessions.
	Sessions []SessionSaveStruct `json:"sessions"`
	// CurrentSessionIndex is the index of the currently loaded session.
	CurrentSessionIndex int `json:"currentSessionIndex"`
}

// NewGameData contains parameters for creating a new game session.
type NewGameData struct {
	// SessionName is the name of the session to create.
	SessionName string `json:"sessionName"`
	// MapName is the path to the map package to use. Empty for default.
	MapName string `json:"mapName"`
	// StartingLocation is the starting location name. Empty for random.
	StartingLocation string `json:"startingLocation"`
	// BSkipOnboarding controls whether onboarding is skipped.
	BSkipOnboarding bool `json:"bSkipOnboarding"`
	// AdvancedGameSettings are the settings to apply to the new session.
	AdvancedGameSettings *AdvancedGameSettings `json:"advancedGameSettings,omitempty"`
	// CustomOptionsOnlyForModding passes custom URL options (not used by vanilla servers).
	CustomOptionsOnlyForModding map[string]string `json:"customOptionsOnlyForModding,omitempty"`
}

// --- internal request envelope types ---

type apiRequest struct {
	Function string `json:"function"`
	Data     any    `json:"data,omitempty"`
}
