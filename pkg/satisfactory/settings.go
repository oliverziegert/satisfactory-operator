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

// GetServerOptions retrieves the currently applied server options and pending options.
// Requires authentication.
func (c *Client) GetServerOptions(ctx context.Context) (*GetServerOptionsResponse, error) {
	body := apiRequest{Function: "GetServerOptions"}
	var result GetServerOptionsResponse
	if err := c.doRequest(ctx, "GetServerOptions", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ApplyServerOptions applies new server option values.
// options is a map of option names to their new stringified values.
// Requires Administrator privilege.
func (c *Client) ApplyServerOptions(ctx context.Context, options map[string]string) error {
	body := apiRequest{
		Function: "ApplyServerOptions",
		Data:     map[string]any{"updatedServerOptions": options},
	}
	return c.doRequest(ctx, "ApplyServerOptions", body, nil)
}

// GetAdvancedGameSettings retrieves the currently applied advanced game settings.
// Requires authentication.
func (c *Client) GetAdvancedGameSettings(ctx context.Context) (*GetAdvancedGameSettingsResponse, error) {
	body := apiRequest{Function: "GetAdvancedGameSettings"}
	var result GetAdvancedGameSettingsResponse
	if err := c.doRequest(ctx, "GetAdvancedGameSettings", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ApplyAdvancedGameSettings applies new advanced game setting values.
// settings is a map of setting names to their new stringified values.
// Advanced Game Settings are automatically enabled if not already active.
func (c *Client) ApplyAdvancedGameSettings(ctx context.Context, settings map[string]string) error {
	body := apiRequest{
		Function: "ApplyAdvancedGameSettings",
		Data:     AdvancedGameSettings{AppliedAdvancedGameSettings: settings},
	}
	return c.doRequest(ctx, "ApplyAdvancedGameSettings", body, nil)
}
