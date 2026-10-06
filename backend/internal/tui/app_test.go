package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"

	"projemble/internal/catalog"
	"projemble/internal/llm"
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

func TestRepairPathInputRequiresExistingAbsoluteDirectory(t *testing.T) {
	input := widgets.NewInput()
	input.Text = "relative"
	var next page
	advance, _, message := handleTextInput(ui.Event{ID: "<Enter>"}, repairPathPage, widgets.NewInput(), widgets.NewInput(), input, widgets.NewInput(), widgets.NewInput(), &next)
	if advance || message == "" {
		t.Fatal("relative path unexpectedly accepted")
	}
	input.Text = t.TempDir()
	advance, _, message = handleTextInput(ui.Event{ID: "<Enter>"}, repairPathPage, widgets.NewInput(), widgets.NewInput(), input, widgets.NewInput(), widgets.NewInput(), &next)
	if !advance || message != "" || next != homePage {
		t.Fatalf("existing directory repair: advance=%v message=%q next=%v", advance, message, next)
	}
}

func TestEscapeLeavesAPIKeyPage(t *testing.T) {
	key := widgets.NewInput()
	key.Text = "partially-entered"
	for _, id := range []string{"<Escape>", "<Esc>", "<Key:Escape>", "<Key:27>", "Escape", "<C-[>", "<C-b>", "\x1b"} {
		var next page
		advance, quit, message := handleTextInput(ui.Event{ID: id}, apiKeyPage, widgets.NewInput(), widgets.NewInput(), widgets.NewInput(), key, widgets.NewInput(), &next)
		if !advance || quit || message != "" || next != providerPage {
			t.Fatalf("Escape %q: advance=%v quit=%v message=%q next=%v", id, advance, quit, message, next)
		}
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

func TestChatGPTSignInTimeoutMessageIsActionable(t *testing.T) {
	if got := chatGPTSignInMessage(context.DeadlineExceeded); got != "ChatGPT sign-in timed out" {
		t.Fatalf("timeout message = %q", got)
	}
	if got := chatGPTSignInMessage(errors.New("sign-in was declined or cancelled")); got != "ChatGPT sign-in failed: sign-in was declined or cancelled" {
		t.Fatalf("authorization error message = %q", got)
	}
}

func TestGenerationReporterStreamsActivityLines(t *testing.T) {
	updates := make(chan generationUpdate, 2)
	writer := generationReporter{ctx: context.Background(), updates: updates, secret: "secret-key"}
	input := []byte("Reading internal/app.go secret-key\nWriting internal/app.go\n")
	written, err := writer.Write(input)
	if err != nil || written != len(input) {
		t.Fatalf("write progress: bytes=%d err=%v", written, err)
	}
	for _, want := range []string{"Reading internal/app.go [REDACTED]", "Writing internal/app.go"} {
		if got := (<-updates).activity; got != want {
			t.Fatalf("activity = %q, want %q", got, want)
		}
	}
	rows := []string(nil)
	for i := 0; i < maxActivityRows+5; i++ {
		rows = appendActivity(rows, fmt.Sprintf("event %d", i))
	}
	if len(rows) != maxActivityRows || rows[0] != "event 5" || rows[len(rows)-1] != "event 304" {
		t.Fatalf("activity history is not bounded correctly: len=%d first=%q last=%q", len(rows), rows[0], rows[len(rows)-1])
	}
}

func TestGenerationReporterStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer := generationReporter{ctx: ctx, updates: make(chan generationUpdate)}
	if written, err := writer.Write([]byte("waiting for UI")); written != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled progress write: bytes=%d err=%v", written, err)
	}
}

func TestActivityRowsUseDistinctSemanticColors(t *testing.T) {
	for _, test := range []struct {
		line, want string
		color      ui.Color
	}{
		{"Waiting on mistral model test-model (turn 1)", "[THINK](fg:" + colorThinking, ui.ColorViolet},
		{"Created internal/app.go", "[+](fg:" + colorAdded, ui.ColorGreen},
		{"Updated internal/app.go", "[~](fg:" + colorChanged, ui.ColorGold},
		{"Removed internal/app.go", "[-](fg:" + colorRemoved, ui.ColorTomato},
		{"You: add a health endpoint", "[YOU](fg:" + colorUser, ui.ColorLightCyan},
		{"tool error: unknown tool", "[ERROR](fg:" + colorError, ui.ColorRed},
	} {
		got := colorActivity(test.line)
		if !strings.HasPrefix(got, test.want) {
			t.Errorf("colorActivity(%q) = %q, want prefix %q", test.line, got, test.want)
		}
		cells := ui.ParseStyles(got, ui.NewStyle(ui.ColorWhite))
		if len(cells) == 0 {
			t.Fatalf("colorActivity(%q) rendered no cells", test.line)
		}
		if cells[0].Style.Fg != test.color {
			t.Errorf("colorActivity(%q) first color = %v, want %v", test.line, cells[0].Style.Fg, test.color)
		}
	}
}

func TestTUIThemePreservesTerminalForegroundAndRestores(t *testing.T) {
	previousDefault, previousTitle := ui.Theme.Default, ui.Theme.Block.Title
	restore := activateTUITheme()
	defer restore()
	if ui.Theme.Default.Fg != ui.ColorClear || ui.Theme.Paragraph.Text.Fg != ui.ColorClear || ui.Theme.List.Text.Fg != ui.ColorClear {
		t.Fatal("base text should inherit the terminal's foreground color")
	}
	if ui.Theme.Block.Title.Fg != ui.ColorLightCyan {
		t.Fatalf("title accent = %v, want light cyan", ui.Theme.Block.Title.Fg)
	}
	restore()
	if ui.Theme.Default != previousDefault || ui.Theme.Block.Title != previousTitle {
		t.Fatal("TUI theme was not restored after the app exits")
	}
}

func TestAgentWorkspaceShowsModelTokenUsageAndPromptInput(t *testing.T) {
	workspace := newAgentWorkspace()
	workspace.AddUsage(llm.Usage{Available: true, InputTokens: 12_345, OutputTokens: 678, TotalTokens: 13_023})
	status := workspace.statusText(filepath.Join(t.TempDir(), "demo"), false, 0)
	for _, want := range []string{"12,345 input", "678 output", "latest request used 12,345 input tokens", "model window limit unavailable"} {
		if !strings.Contains(status, want) {
			t.Errorf("workspace status missing %q:\n%s", want, status)
		}
	}
	options := generationOptions{Mode: "agent", Provider: "mistral", Model: "mistral-test-model"}
	workspace.Render(120, 36, []string{"Writing internal/server.go"}, options, ".", true, true, 2)
	if !strings.Contains(workspace.header.Text, "Mistral") || !strings.Contains(workspace.header.Text, "mistral-test-model") {
		t.Fatalf("selected provider/model missing from header: %q", workspace.header.Title)
	}
	if !strings.Contains(workspace.header.Text, "Agent is thinking") {
		t.Fatalf("working status missing from header: %q", workspace.header.Text)
	}
	frame := workspace.header.Text
	workspace.Tick()
	if workspace.statusText(".", true, 2) == frame {
		t.Fatal("agent spinner did not advance")
	}
	if !workspace.composer.ShowCursor || !strings.Contains(workspace.composer.TitleBottom, "queued 2/") {
		t.Fatalf("busy composer should accept and show queued prompts: cursor=%v footer=%q", workspace.composer.ShowCursor, workspace.composer.TitleBottom)
	}
	for _, char := range "Add a health endpoint" {
		workspace.Handle(ui.Event{Type: ui.KeyboardEvent, ID: string(char)}, false)
	}
	workspace.Handle(ui.Event{Type: ui.KeyboardEvent, ID: "q"}, false)
	workspace.Handle(ui.Event{Type: ui.KeyboardEvent, ID: "b"}, false)
	if !strings.HasSuffix(workspace.composer.Text, "qb") {
		t.Fatalf("printable command letters were intercepted: %q", workspace.composer.Text)
	}
	action := workspace.Handle(ui.Event{ID: "<Enter>"}, false)
	if action.prompt != "Add a health endpointqb" || workspace.composer.Text != "" {
		t.Fatalf("submitted prompt = %q, input remaining = %q", action.prompt, workspace.composer.Text)
	}
	workspace.Handle(ui.Event{ID: "<C-j>"}, false)
	if !strings.Contains(workspace.composer.Text, "\n") {
		t.Fatal("Ctrl+J did not insert a new line")
	}
}

func TestAgentWorkspaceAcceptsAndQueuesPromptWhileBusy(t *testing.T) {
	workspace := newAgentWorkspace()
	for _, char := range "Add a health endpoint" {
		workspace.Handle(ui.Event{Type: ui.KeyboardEvent, ID: string(char)}, true)
	}
	action := workspace.Handle(ui.Event{ID: "<Enter>"}, true)
	if action.prompt != "Add a health endpoint" || !action.queued {
		t.Fatalf("busy prompt action = %+v, want queued prompt", action)
	}
	if workspace.composer.Text != "" {
		t.Fatalf("queued prompt remained in composer: %q", workspace.composer.Text)
	}
}

func TestCancelledGenerationDoesNotCreateProjectDirectory(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	parent := t.TempDir()
	path := filepath.Join(parent, "cancelled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	updates := make(chan generationUpdate, 1)
	_, _, err := saveProjectWithProgress(ctx, "cancelled", "test", path, catalog.Templates()[0], generationOptions{Mode: "local"}, generationReporter{ctx: ctx, updates: updates})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled generation error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled generation left project directory behind: %v", err)
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
	config.ProviderKeys = map[string]string{"openai": "secret-provider-key"}
	if err := projectstore.Save(path, config); err != nil {
		t.Fatal(err)
	}
	contents, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "provider_keys:") || !strings.Contains(string(contents), "secret-provider-key") {
		t.Fatalf("provider credentials were not serialized to YAML: %s", contents)
	}
}
