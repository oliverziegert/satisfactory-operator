package main

import (
	"slices"
	"testing"
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
