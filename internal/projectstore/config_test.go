package projectstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"projemble/internal/catalog"
)

func TestConfigV1LegacyProjectLoadsWithProfileDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	legacy := `version: 1
projects:
  - id: old-project
    name: legacy
    path: C:/work/legacy
    stack: go
    app_shape: monolith
    architecture: layered
    template: go-monolith-layered
    created_at: 2026-01-01T00:00:00Z
    updated_at: 2026-01-01T00:00:00Z
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	project := config.Projects[0]
	if project.WorkloadID != catalog.WorkloadHTTPAPI || project.TopologyID != catalog.TopologyMonolith {
		t.Fatalf("legacy profile defaults: workload=%q topology=%q", project.WorkloadID, project.TopologyID)
	}
	if project.PatternIDs == nil || project.Capabilities == nil {
		t.Fatal("legacy selections should normalize to empty slices")
	}
}

func TestComposedProfileSelectionsPersistAndRejectPlannedTools(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	config := NewConfig()
	project := Project{Name: "rag-api", Path: "C:/work/rag-api", TemplateID: catalog.TemplateGoMonolithClean, PatternIDs: []string{catalog.PatternRAG, catalog.PatternChatbot}, Capabilities: []string{catalog.CapabilityPostgres}}
	if err := config.Upsert(project); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, config); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := loaded.Projects[0]
	if got.WorkloadID != catalog.WorkloadHTTPAPI || got.TopologyID != catalog.TopologyMonolith || strings.Join(got.PatternIDs, ",") != "rag,chatbot" || strings.Join(got.Capabilities, ",") != catalog.CapabilityPostgres {
		t.Fatalf("profile selections failed to round-trip: %+v", got)
	}
	got.Capabilities = []string{catalog.CapabilityRedis}
	if err := loaded.Upsert(got); err == nil {
		t.Fatal("planned capability was accepted")
	}
}

func TestGenerationDefaultsSurviveConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	want := NewConfig()
	want.AgentAccessMode = "ask-always"
	want.Generation = GenerationDefaults{Mode: "agent", Provider: "claude", Model: "claude-test-model", ReasoningEffort: "high"}
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
	if got.AgentAccessMode != want.AgentAccessMode {
		t.Fatalf("loaded agent access mode = %q, want %q", got.AgentAccessMode, want.AgentAccessMode)
	}
	if got.ProviderKeys["claude"] != "test-secret" {
		t.Fatalf("provider key did not round-trip: %q", got.ProviderKeys["claude"])
	}
}

func TestConfigRejectsUnknownAgentAccessMode(t *testing.T) {
	config := NewConfig()
	config.AgentAccessMode = "unrestricted"
	if err := config.Validate(); err == nil {
		t.Fatal("unknown agent access mode was accepted")
	}
}

func TestConfigAcceptsSupportedAgentAccessModes(t *testing.T) {
	for _, mode := range []string{"read-only", "full-access", "ask-always"} {
		config := NewConfig()
		config.AgentAccessMode = mode
		if err := config.Validate(); err != nil {
			t.Errorf("access mode %q was rejected: %v", mode, err)
		}
	}
}
