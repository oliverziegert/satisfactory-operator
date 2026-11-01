package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/oliverziegert/satisfactory-operator/internal/backup"
)

func TestWatchNamespaces(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		envValue  string
		want      []string
	}{
		{name: "flag", flagValue: "games,games-test", want: []string{"games", "games-test"}},
		{name: "env fallback", envValue: " games , ", want: []string{"games"}},
		{name: "flag wins over env", flagValue: "games", envValue: "other", want: []string{"games"}},
		{name: "whitespace", flagValue: " games ,\tgames-test ", want: []string{"games", "games-test"}},
		{name: "empty", want: nil},
		{name: "only separators", flagValue: " , ,", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := watchNamespaces(tt.flagValue, tt.envValue); !slices.Equal(got, tt.want) {
				t.Errorf("watchNamespaces(%q, %q) = %q, want %q", tt.flagValue, tt.envValue, got, tt.want)
			}
		})
	}
}

func TestRunBackup(t *testing.T) {
	var auth []string
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		switch r.URL.Query().Get("function") {
		case "SaveGame":
			w.WriteHeader(http.StatusNoContent)
		case "DownloadSaveGame":
			_, _ = w.Write([]byte("bytes"))
		case "EnumerateSessions":
			_, _ = w.Write([]byte(`{"data":{"sessions":[]}}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()
	dir := t.TempDir()
	tokenFile, resultFile := filepath.Join(dir, "token"), filepath.Join(dir, "result")
	if err := os.WriteFile(tokenFile, []byte("tok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--server-url=" + ts.URL, "--save-name=f_20261101-120000", "--session-prefix=f",
		`--config=[{"name":"server","server":{}}]`, "--token-file=" + tokenFile}

	if code := runBackup(context.Background(), args, resultFile); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	b, _ := os.ReadFile(resultFile)
	var res backup.Result
	if err := json.Unmarshal(b, &res); err != nil || !res.OK() || res.Size != 5 || len(res.Destinations) != 1 {
		t.Fatalf("unexpected result %s (%v)", b, err)
	}
	if auth[0] != "Bearer tok" {
		t.Errorf("expected bearer token from file, got %q", auth[0])
	}

	if code := runBackup(context.Background(), append(slices.Clip(args), "--config=not json"), resultFile); code != 1 {
		t.Errorf("bad config: exit code %d, want 1", code)
	}
}
