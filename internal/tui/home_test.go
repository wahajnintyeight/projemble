package tui

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"projemble/internal/catalog"
	"projemble/internal/llm"
	"projemble/internal/projectstore"
	"testing"
)

func TestHomeLoadsProjectsDirectoryAndCredentials(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("MISTRAL_API_KEY", "environment-key")
	config := projectstore.NewConfig()
	config.ProviderKeys = map[string]string{"mistral": "saved-key"}
	if got := rememberedKey(llm.Mistral, config); got != "saved-key" {
		t.Fatalf("key = %q", got)
	}
	if err := saveProviderKey(llm.Mistral, "roundtrip-key"); err != nil {
		t.Fatal(err)
	}
	loaded, err := projectstore.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	if got := rememberedKey(llm.Mistral, loaded); got != "roundtrip-key" {
		t.Fatalf("stored key = %q", got)
	}
	parent := t.TempDir()
	config = projectstore.NewConfig()
	config.Projects = []projectstore.Project{{Name: "saved", Path: filepath.Join(parent, "saved"), Status: "interrupted"}}
	if got := rememberedDirectory(config); got != parent {
		t.Fatalf("directory = %q", got)
	}
	if got := homeChoices(config); len(got) != 2 || got[1].name != "saved" {
		t.Fatalf("home = %+v", got)
	}
}

func TestFailedAgentSetupPreservesScaffoldAndProfile(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	parent := t.TempDir()
	path := filepath.Join(parent, "preserved")
	_, _, err := saveProjectWithProgress(context.Background(), "preserved", "example", path, catalog.Templates()[0], generationOptions{Mode: "agent", Provider: "unsupported-provider", Model: "test", APIKey: "dummy"}, io.Discard)
	if err == nil {
		t.Fatal("invalid provider unexpectedly succeeded")
	}
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err != nil {
		t.Fatalf("scaffold lost: %v", err)
	}
	config, err := projectstore.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Projects) != 1 || config.Projects[0].Status != "interrupted" || config.ParentDirectory != parent {
		t.Fatalf("profile not recoverable: %+v", config)
	}
}

func TestRepairProjectDirectoryUpdatesSavedProfile(t *testing.T) {
	config := projectstore.NewConfig()
	template := catalog.Templates()[0]
	missingPath := filepath.Join(t.TempDir(), "old-location")
	if err := config.Upsert(projectstore.Project{Name: "recover", Path: missingPath, StackID: template.StackID, AppShapeID: template.AppShapeID, ArchitectureID: template.ArchitectureID, TemplateID: template.ID}); err != nil {
		t.Fatal(err)
	}
	projectID := config.Projects[0].ID
	newPath := t.TempDir()
	if err := repairProjectDirectory(&config, projectID, newPath); err != nil {
		t.Fatal(err)
	}
	if config.Projects[0].Path != newPath || config.LastProjectID != projectID {
		t.Fatalf("repaired profile = %+v", config.Projects[0])
	}
	if err := repairProjectDirectory(&config, projectID, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing replacement directory was accepted")
	}
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := repairProjectDirectory(&config, projectID, file); err == nil {
		t.Fatal("file path was accepted as a project directory")
	}
}
