package tui

import (
	"strings"
	"testing"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/llm/factory"
	"projemble/internal/projectstore"
)

func TestModelPickerSearchFiltersAndRestoresNavigation(t *testing.T) {
	p := newModelPicker()
	p.models = []factory.ModelOption{
		{ID: "mistral-large-latest", DisplayName: "mistral-large-latest"},
		{ID: "mistral-small-latest", DisplayName: "mistral-small-latest"},
		{ID: "open-mistral-nemo", DisplayName: "open-mistral-nemo"},
	}
	p.message = "3 models"
	input := widgets.NewInput()
	p.list.Rows = []string{"Enter a custom model ID", "mistral-large-latest", "mistral-small-latest", "open-mistral-nemo"}
	p.list.SelectedRow = 2
	p.handle(ui.Event{ID: "/"}, input)
	for _, key := range "large" {
		p.handle(ui.Event{ID: string(key), Type: ui.KeyboardEvent}, input)
	}
	p.render(input, 100, 30, "")
	if len(p.list.Rows) != 2 || p.list.Rows[1] != "mistral-large-latest" {
		t.Fatalf("search results: %v", p.list.Rows)
	}
	if !p.querying || p.search.Text != "large" {
		t.Fatal("search input is not active")
	}
	p.handle(ui.Event{ID: "<Enter>"}, input)
	if p.querying {
		t.Fatal("Enter did not return focus to result list")
	}
	p.list.SelectedRow = 1
	_, consumed := p.handle(ui.Event{ID: "<Enter>"}, input)
	if consumed || input.Text != "mistral-large-latest" {
		t.Fatalf("filtered choice=%q consumed=%v", input.Text, consumed)
	}
	p.querying = true
	p.handle(ui.Event{ID: "<Escape>"}, input)
	if p.query != "" || p.querying {
		t.Fatal("Escape should clear search and return to model list")
	}
}

func TestWorkspaceResponsiveSidebarAndNavigation(t *testing.T) {
	w := newAgentWorkspace()
	options := generationOptions{Provider: "mistral", Model: "example"}
	w.Render(160, 40, nil, options, "example", false, true, 0)
	if w.header.Min.X == 0 || w.transcript.Min.Y != 0 || w.composer.Max.X != w.header.Min.X {
		t.Fatal("wide layout did not place session rail on right")
	}
	w.Render(80, 24, nil, options, "example", false, true, 0)
	if w.header.Min.X != 0 || w.transcript.Min.Y == 0 {
		t.Fatal("narrow layout did not stack components")
	}
	w.composer.Text = "keep draft"
	for _, key := range []string{"<F2>", "<F3>", "<F4>", "<Escape>"} {
		a := w.Handle(ui.Event{ID: key}, true)
		if !a.back && !a.provider && !a.model {
			t.Fatalf("shortcut %s ignored during work", key)
		}
	}
	if w.composer.Text != "keep draft" {
		t.Fatal("navigation consumed draft")
	}
	if workspaceDestination(workspaceAction{provider: true}) != providerPage || workspaceDestination(workspaceAction{model: true}) != aiModelPage {
		t.Fatal("incorrect navigation route")
	}
}

func TestChatGPTWorkspaceShowsThinkingControl(t *testing.T) {
	w := newAgentWorkspace()
	options := generationOptions{Provider: "openai-web", Model: "gpt-5.6-luna", ReasoningEffort: "high"}
	w.Render(160, 40, nil, options, ".", false, true, 0)
	if !strings.Contains(w.header.Text, "Thinking effort:") || !strings.Contains(w.header.Text, "high") || !strings.Contains(w.navigation.Text, "F5  Thinking effort") || !strings.Contains(w.composer.TitleBottom, "F5 thinking") {
		t.Fatalf("thinking control is missing from the workspace: header=%q navigation=%q footer=%q", w.header.Text, w.navigation.Text, w.composer.TitleBottom)
	}
	if action := w.Handle(ui.Event{ID: "<F5>"}, false); !action.thinking {
		t.Fatal("F5 did not open the thinking effort control")
	}
}

func TestModelPickerSelectsListedAndCustomModels(t *testing.T) {
	p := newModelPicker()
	p.models = []factory.ModelOption{
		{ID: "model-a", DisplayName: "model-a"},
		{ID: "model-b", DisplayName: "model-b"},
	}
	input := widgets.NewInput()
	p.list.SelectedRow = 2
	p.render(input, 100, 30, "")
	_, consumed := p.handle(ui.Event{ID: "<Enter>"}, input)
	if consumed || input.Text != "model-b" {
		t.Fatal("listed model did not populate confirmation")
	}
	p.list.SelectedRow = 0
	_, consumed = p.handle(ui.Event{ID: "<Enter>"}, input)
	if !consumed || !p.custom {
		t.Fatal("custom model entry unavailable")
	}
	p.handle(ui.Event{ID: "<Escape>"}, input)
	if p.custom {
		t.Fatal("custom entry could not return to list")
	}
	config := projectstore.Config{Projects: []projectstore.Project{{ID: "one", Path: "one"}}}
	project, err := projectForWorkspace(config, "one")
	if err != nil || project.ID != "one" {
		t.Fatal("workspace lost saved project")
	}
	if _, err := projectForWorkspace(config, "missing"); err == nil {
		t.Fatal("missing profile accepted")
	}
}
