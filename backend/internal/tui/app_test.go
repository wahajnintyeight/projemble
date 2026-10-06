package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

func TestWizardInputValidationAndNavigation(t *testing.T) {
	nameInput, descriptionInput := widgets.NewInput(), widgets.NewInput()
	locationInput := widgets.NewInput()
	keyInput, modelInput := widgets.NewInput(), widgets.NewInput()
	var nextPage page

	advance, quit, message := handleTextInput(ui.Event{ID: "<Enter>"}, projectNamePage, nameInput, descriptionInput, locationInput, keyInput, modelInput, &nextPage)
	if advance || quit || message == "" {
		t.Fatalf("empty project name: advance=%v quit=%v message=%q", advance, quit, message)
	}
	for _, r := range "CON" {
		nameInput.InsertRune(r)
	}
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectNamePage, nameInput, descriptionInput, locationInput, keyInput, modelInput, &nextPage)
	if advance || quit || message == "" {
		t.Fatalf("reserved Windows project name accepted: advance=%v quit=%v message=%q", advance, quit, message)
	}
	nameInput.Text = "inventory-api"
	nameInput.Cursor = len(nameInput.Text)
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectNamePage, nameInput, descriptionInput, locationInput, keyInput, modelInput, &nextPage)
	if !advance || quit || message != "" || nextPage != projectDescriptionPage {
		t.Fatalf("valid project name: advance=%v quit=%v message=%q next=%v", advance, quit, message, nextPage)
	}
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectDescriptionPage, nameInput, descriptionInput, locationInput, keyInput, modelInput, &nextPage)
	if advance || quit || message == "" {
		t.Fatalf("empty description: advance=%v quit=%v message=%q", advance, quit, message)
	}
	descriptionInput.Text = "Tracks stock levels"
	descriptionInput.Cursor = len(descriptionInput.Text)
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectDescriptionPage, nameInput, descriptionInput, locationInput, keyInput, modelInput, &nextPage)
	if !advance || quit || message != "" || nextPage != projectLocationPage {
		t.Fatalf("valid description: advance=%v quit=%v message=%q next=%v", advance, quit, message, nextPage)
	}
	locationInput.Text = t.TempDir()
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectLocationPage, nameInput, descriptionInput, locationInput, keyInput, modelInput, &nextPage)
	if !advance || quit || message != "" || nextPage != appShapePage {
		t.Fatalf("valid project location: advance=%v quit=%v message=%q next=%v", advance, quit, message, nextPage)
	}
}

func TestProjectLocationValidationAndPlannedPath(t *testing.T) {
	parent := t.TempDir()
	gotParent, err := validateProjectParent(parent)
	if err != nil || gotParent != filepath.Clean(parent) {
		t.Fatalf("existing absolute directory rejected: parent=%q err=%v", gotParent, err)
	}
	if _, err := validateProjectParent("relative/path"); err == nil {
		t.Fatal("relative project location was accepted")
	}
	missing := filepath.Join(parent, "does-not-exist")
	if _, err := validateProjectParent(missing); err == nil {
		t.Fatal("nonexistent project location was accepted")
	}
	file := filepath.Join(parent, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := validateProjectParent(file); err == nil {
		t.Fatal("file was accepted as project location")
	}
	planned, err := plannedProjectPath(parent, "sample-app")
	if err != nil || planned != filepath.Join(parent, "sample-app") {
		t.Fatalf("planned path=%q err=%v", planned, err)
	}
	if err := os.Mkdir(planned, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := plannedProjectPath(parent, "sample-app"); err == nil {
		t.Fatal("existing project directory was accepted")
	}
}

func TestAISetupRequiresMaskedKeyAndModel(t *testing.T) {
	keyInput, modelInput := widgets.NewInput(), widgets.NewInput()
	var next page
	advance, quit, message := handleTextInput(ui.Event{ID: "<Enter>"}, apiKeyPage, widgets.NewInput(), widgets.NewInput(), widgets.NewInput(), keyInput, modelInput, &next)
	if advance || quit || message == "" {
		t.Fatalf("empty API key accepted: advance=%v quit=%v message=%q", advance, quit, message)
	}
	keyInput.Text = "secret-provider-key"
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, apiKeyPage, widgets.NewInput(), widgets.NewInput(), widgets.NewInput(), keyInput, modelInput, &next)
	if !advance || quit || message != "" || next != aiModelPage {
		t.Fatalf("API key step: advance=%v quit=%v message=%q next=%v", advance, quit, message, next)
	}
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, aiModelPage, widgets.NewInput(), widgets.NewInput(), widgets.NewInput(), keyInput, modelInput, &next)
	if advance || quit || message == "" {
		t.Fatalf("empty model accepted: advance=%v quit=%v message=%q", advance, quit, message)
	}
	modelInput.Text = "test-model"
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, aiModelPage, widgets.NewInput(), widgets.NewInput(), widgets.NewInput(), keyInput, modelInput, &next)
	if !advance || quit || message != "" || next != projectNamePage {
		t.Fatalf("model step: advance=%v quit=%v message=%q next=%v", advance, quit, message, next)
	}
}

func TestProviderCatalogIncludesAPIKeysAndChatGPTSignIn(t *testing.T) {
	choices := providerCatalog()
	if len(choices) != 9 {
		t.Fatalf("provider choices = %d, want 9", len(choices))
	}
	if choices[0].id != "openai" || choices[0].env != "OPENAI_API_KEY" {
		t.Fatalf("OpenAI provider setup = %+v", choices[0])
	}
	if choices[len(choices)-1].id != "openai-web" || choices[len(choices)-1].env != "" {
		t.Fatalf("ChatGPT provider setup = %+v", choices[len(choices)-1])
	}
}

func TestInitialGenerationOptionsLoadsSavedDefaults(t *testing.T) {
	config := projectstore.Config{
		Generation: projectstore.GenerationDefaults{
			Mode: "agent", Provider: "claude", Model: "claude-test-model",
		},
	}
	got := initialGenerationOptions(config)
	if got.Mode != "agent" || got.Provider != "claude" || got.Model != "claude-test-model" {
		t.Fatalf("initial generation options = %+v, want saved AI defaults", got)
	}
}

func TestInitialGenerationOptionsFallsBackToLatestSavedProject(t *testing.T) {
	config := projectstore.Config{Projects: []projectstore.Project{
		{GenerationMode: "local", UpdatedAt: "2026-01-01T00:00:00Z"},
		{GenerationMode: "agent", AIProvider: "gemini", AIModel: "gemini-test", UpdatedAt: "2026-02-01T00:00:00Z"},
	}}
	got := initialGenerationOptions(config)
	if got.Mode != "agent" || got.Provider != "gemini" || got.Model != "gemini-test" {
		t.Fatalf("initial generation options = %+v, want defaults from latest saved project", got)
	}
}

func TestReviewShowsGenerationChoiceWithoutShowingAPIKey(t *testing.T) {
	list := widgets.NewList()
	template := catalog.Templates()[0]
	options := generationOptions{Mode: "agent", Provider: "openai", Model: "gpt-test", APIKey: "secret-provider-key"}
	updateSummary(list, "sample", "description", template, filepath.Join(t.TempDir(), "sample"), options, "Enter generate project", "", 100, 35)
	rows := strings.Join(list.Rows, "\n")
	if !strings.Contains(rows, "AI-assisted · OpenAI · gpt-test") {
		t.Fatalf("generation details missing from review:\n%s", rows)
	}
	if strings.Contains(rows, options.APIKey) {
		t.Fatal("API key was displayed in the review")
	}
	if !strings.Contains(list.TitleBottom, "r provider") {
		t.Fatalf("review does not expose provider reconfiguration shortcut: %q", list.TitleBottom)
	}
}

func TestSaveWizardProfilesForEveryTemplate(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("APPDATA", configRoot)
	parent := t.TempDir()

	for i, template := range catalog.Templates() {
		name := fmt.Sprintf("sample-%d", i)
		path, err := plannedProjectPath(parent, name)
		if err != nil {
			t.Fatalf("plan path for template %s: %v", template.ID, err)
		}
		if _, _, err := saveProject(name, "A test project", path, template); err != nil {
			t.Fatalf("save template %s: %v", template.ID, err)
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("project directory was not created: path=%q info=%v err=%v", path, info, err)
		}
	}

	configPath, err := projectstore.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	config, err := projectstore.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(config.Projects), len(catalog.Templates()); got != want {
		t.Fatalf("saved project count = %d, want %d", got, want)
	}
	for i, template := range catalog.Templates() {
		project := config.Projects[i]
		if project.Name != fmt.Sprintf("sample-%d", i) || project.Description != "A test project" || project.TemplateID != template.ID {
			t.Errorf("saved project %d mismatch: %+v", i, project)
		}
		if project.GenerationMode != "local" {
			t.Errorf("saved project %s mode = %q, want local", project.Name, project.GenerationMode)
		}
		if template.ServiceFrameworkID != "" && project.Settings[catalog.SettingServiceFramework] != template.ServiceFrameworkID {
			t.Errorf("template %s lost service framework setting: %v", template.ID, project.Settings)
		}
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("YAML config was not created: %v", err)
	}
}

func TestAgentGenerationProfileStoresProviderAndModelOnly(t *testing.T) {
	template := catalog.Templates()[0]
	config := projectstore.NewConfig()
	if err := config.Upsert(projectstore.Project{
		Name: "ai-sample", Path: filepath.Join(t.TempDir(), "ai-sample"), StackID: template.StackID,
		AppShapeID: template.AppShapeID, ArchitectureID: template.ArchitectureID, TemplateID: template.ID,
		GenerationMode: "agent", AIProvider: "openai", AIModel: "test-model",
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := projectstore.Save(path, config); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "generation_mode: agent") || !strings.Contains(string(contents), "ai_provider: openai") || !strings.Contains(string(contents), "ai_model: test-model") {
		t.Fatalf("AI generation settings missing from profile:\n%s", contents)
	}
	if strings.Contains(string(contents), "api_key") || strings.Contains(string(contents), "secret-provider-key") {
		t.Fatalf("provider secret was serialized:\n%s", contents)
	}
}
