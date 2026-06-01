package models

import "testing"

func TestGeneratedRegistryIncludesMimoAndMiniMax(t *testing.T) {
	registry := NewModelRegistry()

	cases := []struct {
		name     string
		pattern  string
		provider string
	}{
		{name: "mimo", pattern: "mimo-v2-flash", provider: "mimo"},
		{name: "minimax", pattern: "minimax-m2", provider: "minimax"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, ok := registry.Resolve(tc.pattern)
			if !ok {
				t.Fatalf("Resolve(%q) did not find a generated model entry", tc.pattern)
			}
			if entry.Provider != tc.provider {
				t.Fatalf("provider = %q, want %q", entry.Provider, tc.provider)
			}
			if entry.ContextWindow <= 0 {
				t.Fatalf("context window = %d, want a positive generated window", entry.ContextWindow)
			}
		})
	}
}
