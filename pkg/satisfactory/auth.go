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

import "context"

// passwordKey is the request data key the API uses for passwords.
const passwordKey = "password"

// HealthCheck performs a health check on the Dedicated Server API.
// This endpoint requires no authentication.
// clientCustomData is passed to the server and returned in serverCustomData (used by modded servers).
func (c *Client) HealthCheck(ctx context.Context, clientCustomData string) (*HealthCheckResponse, error) {
	body := apiRequest{
		Function: "HealthCheck",
		Data:     map[string]string{"clientCustomData": clientCustomData},
	}
	var result HealthCheckResponse
	if err := c.doRequest(ctx, "HealthCheck", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// VerifyAuthenticationToken verifies the current bearer token is still valid.
// Returns nil on success (HTTP 204), or *APIError on failure.
func (c *Client) VerifyAuthenticationToken(ctx context.Context) error {
	body := apiRequest{Function: "VerifyAuthenticationToken"}
	return c.doRequest(ctx, "VerifyAuthenticationToken", body, nil)
}

// PasswordlessLogin attempts a passwordless login to the server.
// This endpoint requires no authentication and is possible only on unclaimed servers
// or when no Client Protection Password is set.
func (c *Client) PasswordlessLogin(
	ctx context.Context, minimumPrivilegeLevel PrivilegeLevel,
) (*PasswordlessLoginResponse, error) {
	body := apiRequest{
		Function: "PasswordlessLogin",
		Data:     map[string]any{"minimumPrivilegeLevel": minimumPrivilegeLevel},
	}
	var result PasswordlessLoginResponse
	if err := c.doRequest(ctx, "PasswordlessLogin", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PasswordLogin attempts to log in using either the Admin Password or Client Protection Password.
// This endpoint requires no authentication.
func (c *Client) PasswordLogin(
	ctx context.Context, minimumPrivilegeLevel PrivilegeLevel, password string,
) (*PasswordLoginResponse, error) {
	body := apiRequest{
		Function: "PasswordLogin",
		Data: map[string]any{
			"minimumPrivilegeLevel": minimumPrivilegeLevel,
			passwordKey:             password,
		},
	}
	var result PasswordLoginResponse
	if err := c.doRequest(ctx, "PasswordLogin", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
