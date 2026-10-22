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
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ----------------------------------------------------------------------------
// APIError.Error()
// ----------------------------------------------------------------------------

func TestAPIError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "only ErrorCode produces code-only message",
			err:  &APIError{ErrorCode: "NOT_FOUND"},
			want: "satisfactory API error NOT_FOUND",
		},
		{
			name: "ErrorCode and ErrorMessage produces combined message",
			err:  &APIError{ErrorCode: "NOT_FOUND", ErrorMessage: "save not found"},
			want: "satisfactory API error NOT_FOUND: save not found",
		},
		{
			name: "empty ErrorCode produces trailing-space message",
			err:  &APIError{ErrorCode: ""},
			want: "satisfactory API error ",
		},
		{
			name: "ErrorCode with ErrorData but no ErrorMessage omits message part",
			err: &APIError{
				ErrorCode: "INTERNAL_ERROR",
				ErrorData: map[string]any{"detail": "disk full"},
			},
			want: "satisfactory API error INTERNAL_ERROR",
		},
		{
			name: "ErrorCode with ErrorMessage and ErrorData includes message part",
			err: &APIError{
				ErrorCode:    "INTERNAL_ERROR",
				ErrorMessage: "disk full",
				ErrorData:    map[string]any{"detail": "disk full"},
			},
			want: "satisfactory API error INTERNAL_ERROR: disk full",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.err.Error()
			if got != tc.want {
				t.Errorf("APIError.Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAPIError_ImplementsErrorInterface verifies that *APIError satisfies the
// built-in error interface at compile time and at runtime.
func TestAPIError_ImplementsErrorInterface(t *testing.T) {
	t.Parallel()

	// Compile-time check: assignment to error variable must compile.
	var err error = &APIError{ErrorCode: "SOME_CODE", ErrorMessage: "some message"}

	const want = "satisfactory API error SOME_CODE: some message"
	if got := err.Error(); got != want {
		t.Errorf("error.Error() via interface = %q, want %q", got, want)
	}
}

// ----------------------------------------------------------------------------
// NewClient constructor
// ----------------------------------------------------------------------------

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("returns non-nil client with nil tlsConfig", func(t *testing.T) {
		t.Parallel()
		c := NewClient("https://example.com", nil)
		if c == nil {
			t.Fatal("NewClient returned nil")
		}
	})

	t.Run("sets baseURL correctly", func(t *testing.T) {
		t.Parallel()
		const base = "https://example.com"
		c := NewClient(base, nil)
		if c.baseURL != base {
			t.Errorf("baseURL = %q, want %q", c.baseURL, base)
		}
	})

	t.Run("httpClient is non-nil with nil tlsConfig", func(t *testing.T) {
		t.Parallel()
		c := NewClient("https://example.com", nil)
		if c.httpClient == nil {
			t.Fatal("httpClient is nil, expected non-nil")
		}
	})

	t.Run("authToken is empty initially", func(t *testing.T) {
		t.Parallel()
		c := NewClient("https://example.com", nil)
		if c.authToken != "" {
			t.Errorf("authToken = %q, want empty string", c.authToken)
		}
	})

	t.Run("returns non-nil client with non-nil tlsConfig", func(t *testing.T) {
		t.Parallel()
		tlsCfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // intentional for test
		c := NewClient("https://self-signed.example.com", tlsCfg)
		if c == nil {
			t.Fatal("NewClient returned nil")
		}
	})

	t.Run("httpClient is non-nil with non-nil tlsConfig", func(t *testing.T) {
		t.Parallel()
		tlsCfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // intentional for test
		c := NewClient("https://self-signed.example.com", tlsCfg)
		if c.httpClient == nil {
			t.Fatal("httpClient is nil, expected non-nil")
		}
	})

	t.Run("preserves baseURL with trailing slash", func(t *testing.T) {
		t.Parallel()
		const base = "https://example.com/"
		c := NewClient(base, nil)
		if c.baseURL != base {
			t.Errorf("baseURL = %q, want %q", c.baseURL, base)
		}
	})

	t.Run("accepts empty baseURL without panicking", func(t *testing.T) {
		t.Parallel()
		// NewClient must not panic; the error surfaces later during doRequest.
		c := NewClient("", nil)
		if c == nil {
			t.Fatal("NewClient returned nil for empty baseURL")
		}
		if c.baseURL != "" {
			t.Errorf("baseURL = %q, want empty string", c.baseURL)
		}
	})
}

// ----------------------------------------------------------------------------
// Client.SetAuthToken
// ----------------------------------------------------------------------------

func TestSetAuthToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "sets a non-empty token",
			token: "mytoken",
			want:  "mytoken",
		},
		{
			name:  "sets an empty token (clears the token)",
			token: "",
			want:  "",
		},
		{
			name:  "sets a token with special characters",
			token: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig",
			want:  "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := NewClient("https://example.com", nil)
			c.SetAuthToken(tc.token)
			if c.authToken != tc.want {
				t.Errorf("authToken = %q, want %q", c.authToken, tc.want)
			}
		})
	}
}

// TestSetAuthToken_Overwrite verifies that calling SetAuthToken twice keeps
// only the most recently set value.
func TestSetAuthToken_Overwrite(t *testing.T) {
	t.Parallel()

	c := NewClient("https://example.com", nil)

	c.SetAuthToken("first-token")
	if c.authToken != "first-token" {
		t.Fatalf("after first SetAuthToken: authToken = %q, want %q", c.authToken, "first-token")
	}

	c.SetAuthToken("second-token")
	if c.authToken != "second-token" {
		t.Errorf("after second SetAuthToken: authToken = %q, want %q", c.authToken, "second-token")
	}
}

// TestSetAuthToken_ClearsExisting verifies that setting an empty token after a
// non-empty one effectively clears the stored credential.
func TestSetAuthToken_ClearsExisting(t *testing.T) {
	t.Parallel()

	c := NewClient("https://example.com", nil)
	c.SetAuthToken("initial-token")
	c.SetAuthToken("")

	if c.authToken != "" {
		t.Errorf("authToken = %q after clearing, want empty string", c.authToken)
	}
}

// ----------------------------------------------------------------------------
// Client.parseResponse
// ----------------------------------------------------------------------------

// TestParseResponse_ErrorCodeWithSuccessStatus verifies that an HTTP 200 body
// carrying an errorCode is returned as *APIError instead of success.
func TestParseResponse_ErrorCodeWithSuccessStatus(t *testing.T) {
	t.Parallel()

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errorCode":"missing_params","errorMessage":"Missing parameters"}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec
	_, err := c.QueryServerState(context.Background())

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.ErrorCode != "missing_params" || apiErr.ErrorMessage != "Missing parameters" {
		t.Errorf("got ErrorCode=%q ErrorMessage=%q", apiErr.ErrorCode, apiErr.ErrorMessage)
	}
}
