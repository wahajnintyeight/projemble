package tui

import (
	"image"
	"testing"

	ui "github.com/metaspartan/gotui/v5"
)

func TestSlashPaletteFiltersCompletesAndSubmits(t *testing.T) {
	workspace := newAgentWorkspace()
	workspace.Handle(ui.Event{ID: "/", Type: ui.KeyboardEvent}, false)
	if !workspace.commands.active(workspace.composer) || len(workspace.commands.matches) != len(slashOptions) {
		t.Fatal("typing slash did not open the full command menu")
	}
	workspace.Render(120, 30, nil, generationOptions{}, ".", false, true, 0)
	if len(workspace.commands.list.Rows) != len(slashOptions) {
		t.Fatalf("rendered commands = %v", workspace.commands.list.Rows)
	}
	workspace.Handle(ui.Event{ID: "<Down>"}, false)
	if workspace.commands.matches[workspace.commands.selected].command != "/new" {
		t.Fatal("Down did not select the next command")
	}
	workspace.Handle(ui.Event{ID: "<Tab>"}, false)
	if workspace.composer.Text != "/new " {
		t.Fatalf("Tab completion = %q", workspace.composer.Text)
	}
	action := workspace.Handle(ui.Event{ID: "<Enter>"}, false)
	if action.prompt != "/new" {
		t.Fatalf("completed command submission = %+v", action)
	}
}

func TestSlashPaletteFiltersAndEscapeDismissesWithoutLeavingWorkspace(t *testing.T) {
	workspace := newAgentWorkspace()
	workspace.composer.Text = "/res"
	workspace.composer.Cursor = image.Pt(4, 0)
	if !workspace.commands.active(workspace.composer) || len(workspace.commands.matches) != 1 || workspace.commands.matches[0].command != "/resume" {
		t.Fatalf("filtered commands = %+v", workspace.commands.matches)
	}
	action := workspace.Handle(ui.Event{ID: "<Escape>"}, false)
	if action.back || workspace.commands.active(workspace.composer) {
		t.Fatal("Escape should close the command menu without leaving the workspace")
	}
}
