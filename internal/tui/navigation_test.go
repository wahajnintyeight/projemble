package tui

import (
	"path/filepath"
	"testing"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/catalog"
	"projemble/internal/llm"
	"projemble/internal/projectstore"
)

func TestEscapeAliasesLeavePickerAndWorkspace(t *testing.T) {
	for _, key := range []string{"<Escape>", "<Esc>", "<Key:Escape>", "<Key:27>", "Escape", "<C-[>", "\x1b"} {
		t.Run(key, func(t *testing.T) {
			picker := newModelPicker()
			input := widgets.NewInput()
			event, consumed := picker.handle(ui.Event{ID: key, Type: ui.KeyboardEvent}, input)
			if consumed || event.ID != "<Escape>" {
				t.Fatal("model list swallowed Escape")
			}
			picker.querying, picker.query = true, "gpt"
			picker.handle(ui.Event{ID: key, Type: ui.KeyboardEvent}, input)
			if picker.querying || picker.query != "" {
				t.Fatal("search did not close")
			}
			picker.custom = true
			picker.handle(ui.Event{ID: key}, input)
			if picker.custom {
				t.Fatal("custom model did not close")
			}
			workspace := newAgentWorkspace()
			workspace.composer.Text = "preserve draft"
			if !workspace.Handle(ui.Event{ID: key}, true).back || workspace.composer.Text != "preserve draft" {
				t.Fatal("workspace did not navigate back while preserving draft")
			}
			workspace.mentions.visible = true
			if workspace.Handle(ui.Event{ID: key}, false).back || workspace.mentions.visible {
				t.Fatal("Escape should close mentions before navigating")
			}
		})
	}
	workspace := newAgentWorkspace()
	before := workspace.showSidebar
	if workspace.Handle(ui.Event{ID: "<C-b>"}, false).back || workspace.showSidebar == before {
		t.Fatal("Ctrl+B no longer toggles sidebar")
	}
}

func TestProjectProviderSelectionSurvivesReload(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	config := projectstore.NewConfig()
	template := catalog.Templates()[0]
	for _, name := range []string{"selected", "other"} {
		err := config.Upsert(projectstore.Project{
			Name: name, Path: filepath.Join(t.TempDir(), name), StackID: template.StackID,
			AppShapeID: template.AppShapeID, ArchitectureID: template.ArchitectureID, TemplateID: template.ID,
			GenerationMode: "agent", AIProvider: "mistral", AIModel: "mistral-medium-3",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	config.Generation = projectstore.GenerationDefaults{Mode: "agent", Provider: "openai", Model: "default-model"}
	options, target := homeProviderSelection(config, 1)
	if options.Provider != llm.Mistral || options.Model != "mistral-medium-3" || target.ID != config.Projects[0].ID {
		t.Fatal("home settings did not target highlighted project")
	}
	options = generationOptions{Mode: "agent", Provider: llm.OpenAIWeb, Model: "gpt-selected"}
	if err := saveGenerationSelection(&config, options, target); err != nil {
		t.Fatal(err)
	}
	loaded, err := projectstore.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	restored, _ := homeProviderSelection(loaded, 1)
	if restored.Provider != llm.OpenAIWeb || restored.Model != "gpt-selected" {
		t.Fatal("project reverted to Mistral after reload")
	}
	other, _ := homeProviderSelection(loaded, 2)
	if other.Provider != llm.Mistral || other.Model != "mistral-medium-3" {
		t.Fatal("unselected project changed")
	}
	defaults, target := homeProviderSelection(loaded, 0)
	if target != nil || defaults.Provider != llm.OpenAIWeb {
		t.Fatal("create row did not edit defaults")
	}
	invalid := options
	invalid.Model = ""
	if err := saveGenerationSelection(&loaded, invalid, &loaded.Projects[0]); err == nil {
		t.Fatal("invalid model accepted")
	}
	if loaded.Projects[0].AIModel != "gpt-selected" || loaded.Generation.Model != "gpt-selected" {
		t.Fatal("failed save changed active settings")
	}
}

func TestThinkingEffortCyclesAndPersistsPerProject(t *testing.T) {
	rows := clearThinkingActivity([]string{"Agent summary: done", "Thinking effort set to low for this project."})
	if len(rows) != 1 || rows[0] != "Agent summary: done" {
		t.Fatalf("thinking changes remain in conversation: %v", rows)
	}
	if got := nextReasoningEffort(llm.ReasoningDefault); got != llm.ReasoningLow {
		t.Fatalf("default -> %q", got)
	}
	if got := nextReasoningEffort(llm.ReasoningLow); got != llm.ReasoningMedium {
		t.Fatalf("low -> %q", got)
	}
	if got := nextReasoningEffort(llm.ReasoningMedium); got != llm.ReasoningHigh {
		t.Fatalf("medium -> %q", got)
	}
	if got := nextReasoningEffort(llm.ReasoningHigh); got != llm.ReasoningDefault {
		t.Fatalf("high -> %q", got)
	}
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	template := catalog.Templates()[0]
	config := projectstore.NewConfig()
	if err := config.Upsert(projectstore.Project{Name: "agent", Path: filepath.Join(t.TempDir(), "agent"), StackID: template.StackID, AppShapeID: template.AppShapeID, ArchitectureID: template.ArchitectureID, TemplateID: template.ID, GenerationMode: "agent", AIProvider: string(llm.OpenAIWeb), AIModel: "gpt-5.6-luna"}); err != nil {
		t.Fatal(err)
	}
	options, project := homeProviderSelection(config, 1)
	options.ReasoningEffort = llm.ReasoningHigh
	if err := saveGenerationSelection(&config, options, project); err != nil {
		t.Fatal(err)
	}
	loaded, err := projectstore.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	restored, _ := homeProviderSelection(loaded, 1)
	if restored.ReasoningEffort != llm.ReasoningHigh || loaded.Generation.ReasoningEffort != string(llm.ReasoningHigh) {
		t.Fatalf("reasoning setting did not survive reload: %+v %+v", restored, loaded.Generation)
	}
}
