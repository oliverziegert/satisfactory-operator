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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is a typed HTTP client for the Satisfactory Dedicated Server HTTPS API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
}

// NewClient creates a new Client targeting the given base URL.
// If tlsConfig is nil, the default TLS settings (with certificate verification) are used.
// Pass a *tls.Config with InsecureSkipVerify: true to connect to servers with self-signed certs.
func NewClient(baseURL string, tlsConfig *tls.Config) *Client {
	transport := &http.Transport{}
	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Transport: transport},
	}
}

// SetAuthToken sets the bearer token used for authenticated API calls.
// All subsequent requests will include an Authorization: Bearer <token> header.
func (c *Client) SetAuthToken(token string) {
	c.authToken = token
}

// doRequest sends a POST to /api/v1/?function=<functionName> with the given body serialized as JSON.
// On success (2xx), the response body is decoded into result (if non-nil).
// On error (non-2xx), the response body is decoded into *APIError.
func (c *Client) doRequest(ctx context.Context, functionName string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/?function=%s", c.baseURL, functionName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return c.parseResponse(resp, result)
}

// parseResponse handles the response from the server.
// 2xx with a result target: decodes the JSON "data" wrapper into result.
// 2xx without a result target: discards the body.
// non-2xx: decodes the body into *APIError and returns it.
func (c *Client) parseResponse(resp *http.Response, result any) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if result == nil {
			return nil
		}
		// Responses wrap the payload in {"data": {...}}
		var wrapper struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
			// Some 2xx responses have no body (e.g. 204)
			return nil
		}
		if wrapper.Data == nil {
			return nil
		}
		if err := json.Unmarshal(wrapper.Data, result); err != nil {
			return fmt.Errorf("decode response data: %w", err)
		}
		return nil
	}

	// Non-2xx: attempt to parse as APIError
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("HTTP %d: failed to read error body: %w", resp.StatusCode, err)
	}
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return &apiErr
}
