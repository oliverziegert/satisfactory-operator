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

// QueryServerState retrieves the current state of the Dedicated Server.
// Requires authentication.
func (c *Client) QueryServerState(ctx context.Context) (*QueryServerStateResponse, error) {
	body := apiRequest{Function: "QueryServerState"}
	var result QueryServerStateResponse
	if err := c.doRequest(ctx, "QueryServerState", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ClaimServer claims an unclaimed server, setting its name and admin password.
// Requires InitialAdmin privilege. Returns a new authentication token.
func (c *Client) ClaimServer(ctx context.Context, serverName, adminPassword string) (*ClaimServerResponse, error) {
	body := apiRequest{
		Function: "ClaimServer",
		Data: map[string]string{
			"serverName":    serverName,
			"adminPassword": adminPassword,
		},
	}
	var result ClaimServerResponse
	if err := c.doRequest(ctx, "ClaimServer", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RenameServer renames the Dedicated Server.
// Requires Administrator privilege.
func (c *Client) RenameServer(ctx context.Context, serverName string) error {
	body := apiRequest{
		Function: "RenameServer",
		Data:     map[string]string{"serverName": serverName},
	}
	return c.doRequest(ctx, "RenameServer", body, nil)
}

// SetClientPassword updates the Client Protection Password.
// Pass an empty string to remove the password.
// Requires Administrator privilege.
func (c *Client) SetClientPassword(ctx context.Context, password string) error {
	body := apiRequest{
		Function: "SetClientPassword",
		Data:     map[string]string{passwordKey: password},
	}
	return c.doRequest(ctx, "SetClientPassword", body, nil)
}

// SetAdminPassword updates the Admin Password and returns a new authentication token.
// The token used for this call is invalidated.
// Requires Administrator privilege.
func (c *Client) SetAdminPassword(ctx context.Context, password string) (*SetAdminPasswordResponse, error) {
	body := apiRequest{
		Function: "SetAdminPassword",
		Data:     map[string]string{passwordKey: password},
	}
	var result SetAdminPasswordResponse
	if err := c.doRequest(ctx, "SetAdminPassword", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SetAutoLoadSessionName sets the session that the server loads automatically on startup.
// Does not affect the currently loaded session.
// Requires Administrator privilege.
func (c *Client) SetAutoLoadSessionName(ctx context.Context, sessionName string) error {
	body := apiRequest{
		Function: "SetAutoLoadSessionName",
		Data:     map[string]string{"sessionName": sessionName},
	}
	return c.doRequest(ctx, "SetAutoLoadSessionName", body, nil)
}

// RunCommand executes a console command on the Dedicated Server.
// Requires Administrator privilege.
func (c *Client) RunCommand(ctx context.Context, command string) error {
	body := apiRequest{
		Function: "RunCommand",
		Data:     map[string]string{"command": command},
	}
	return c.doRequest(ctx, "RunCommand", body, nil)
}

// Shutdown shuts down the Dedicated Server.
// If an automatic restart script is configured, this allows applying updates.
// Requires Administrator privilege.
func (c *Client) Shutdown(ctx context.Context) error {
	body := apiRequest{Function: "Shutdown"}
	return c.doRequest(ctx, "Shutdown", body, nil)
}
