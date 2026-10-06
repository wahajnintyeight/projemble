package projectstore

import (
	"path/filepath"
	"testing"
)

func TestGenerationDefaultsSurviveConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	want := NewConfig()
	want.Generation = GenerationDefaults{Mode: "agent", Provider: "claude", Model: "claude-test-model"}
	want.ProviderKeys = map[string]string{"claude": "test-secret"}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Generation != want.Generation {
		t.Fatalf("loaded generation defaults = %+v, want %+v", got.Generation, want.Generation)
	}
	if got.ProviderKeys["claude"] != "test-secret" {
		t.Fatalf("provider key did not round-trip: %q", got.ProviderKeys["claude"])
	}
}
