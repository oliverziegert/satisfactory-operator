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

package satisfactory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

// saveNameKey is the request data key the API uses for save names.
const saveNameKey = "saveName"

// CreateNewGame creates a new game session and immediately loads it.
// The HTTPS API becomes temporarily unavailable while the map loads.
// Requires authentication.
func (c *Client) CreateNewGame(ctx context.Context, data *NewGameData) error {
	body := apiRequest{
		Function: "CreateNewGame",
		Data:     map[string]any{"newGameData": data},
	}
	return c.doRequest(ctx, "CreateNewGame", body, nil)
}

// SaveGame saves the currently loaded session to a named file.
// The save name may be sanitized by the server.
// Requires Administrator privilege.
func (c *Client) SaveGame(ctx context.Context, saveName string) error {
	body := apiRequest{
		Function: "SaveGame",
		Data:     map[string]string{saveNameKey: saveName},
	}
	return c.doRequest(ctx, "SaveGame", body, nil)
}

// LoadGame loads a save game file by name.
// The HTTPS API becomes temporarily unavailable while the save is loading.
// Requires Administrator privilege.
func (c *Client) LoadGame(ctx context.Context, saveName string, enableAdvancedGameSettings bool) error {
	body := apiRequest{
		Function: "LoadGame",
		Data: map[string]any{
			saveNameKey:                  saveName,
			"enableAdvancedGameSettings": enableAdvancedGameSettings,
		},
	}
	return c.doRequest(ctx, "LoadGame", body, nil)
}

// DeleteSaveFile deletes a single save game file from the server.
// Requires Administrator privilege.
func (c *Client) DeleteSaveFile(ctx context.Context, saveName string) error {
	body := apiRequest{
		Function: "DeleteSaveFile",
		Data:     map[string]string{saveNameKey: saveName},
	}
	return c.doRequest(ctx, "DeleteSaveFile", body, nil)
}

// DeleteSaveSession deletes all save files belonging to a session.
// Requires Administrator privilege.
func (c *Client) DeleteSaveSession(ctx context.Context, sessionName string) error {
	body := apiRequest{
		Function: "DeleteSaveSession",
		Data:     map[string]string{"sessionName": sessionName},
	}
	return c.doRequest(ctx, "DeleteSaveSession", body, nil)
}

// EnumerateSessions lists all save sessions and their save files on the server.
// Requires Administrator privilege.
func (c *Client) EnumerateSessions(ctx context.Context) (*EnumerateSessionsResponse, error) {
	body := apiRequest{Function: "EnumerateSessions"}
	var result EnumerateSessionsResponse
	if err := c.doRequest(ctx, "EnumerateSessions", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UploadSaveGame uploads a save game file to the server using multipart/form-data.
// If loadSaveGame is true, the server loads the file immediately after upload.
// Requires Administrator privilege.
func (c *Client) UploadSaveGame(
	ctx context.Context, saveName string, loadSaveGame, enableAdvancedGameSettings bool, saveFileReader io.Reader,
) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// JSON "data" part with explicit Content-Type: application/json
	dataPart, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": []string{`form-data; name="data"`},
		"Content-Type":        []string{"application/json"},
	})
	if err != nil {
		return fmt.Errorf("create data part: %w", err)
	}
	envelope := map[string]any{
		"function": "UploadSaveGame",
		"data": map[string]any{
			saveNameKey:                  saveName,
			"loadSaveGame":               loadSaveGame,
			"enableAdvancedGameSettings": enableAdvancedGameSettings,
		},
	}
	if err := json.NewEncoder(dataPart).Encode(envelope); err != nil {
		return fmt.Errorf("encode data part: %w", err)
	}

	// Binary "saveGameFile" part
	filePart, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": []string{`form-data; name="saveGameFile"; filename="save.sav"`},
		"Content-Type":        []string{"application/octet-stream"},
	})
	if err != nil {
		return fmt.Errorf("create file part: %w", err)
	}
	if _, err := io.Copy(filePart, saveFileReader); err != nil {
		return fmt.Errorf("write save file: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/?function=UploadSaveGame", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute upload request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return c.parseResponse(resp, nil)
}

// DownloadSaveGame downloads a save game file from the server.
// Returns an io.ReadCloser that streams the binary save data.
// The caller is responsible for closing the returned reader.
// Requires Administrator privilege.
func (c *Client) DownloadSaveGame(ctx context.Context, saveName string) (io.ReadCloser, error) {
	reqBody := apiRequest{
		Function: "DownloadSaveGame",
		Data:     map[string]string{saveNameKey: saveName},
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/?function=DownloadSaveGame", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute download request: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Successful download — return the body as a streaming reader
		return resp.Body, nil
	}

	// Error response — read and parse
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP %d: failed to read error body: %w", resp.StatusCode, err)
	}
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil, &apiErr
}
