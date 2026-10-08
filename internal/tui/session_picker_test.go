package tui

import (
	"strings"
	"testing"
	"time"

	ui "github.com/metaspartan/gotui/v5"
	"projemble/internal/agent"
	"projemble/internal/llm"
)

func TestSessionPickerShowsAllAndResumesSelection(t *testing.T) {
	activeID := strings.Repeat("a", 32)
	otherID := strings.Repeat("b", 32)
	picker := newSessionPicker()
	picker.open([]agent.SessionInfo{
		{ID: activeID, Title: "Current work", UpdatedAt: time.Now()},
		{ID: otherID, Title: "Older work", UpdatedAt: time.Now().Add(-time.Hour)},
	}, activeID)
	if !picker.visible || len(picker.list.Rows) != 2 || !strings.HasPrefix(picker.list.Rows[0], "* Current work") {
		t.Fatalf("picker state = visible %v, rows %v", picker.visible, picker.list.Rows)
	}
	picker.handle(ui.Event{ID: "<Down>"})
	action := picker.handle(ui.Event{ID: "<Enter>"})
	if action.prompt != commandResume+" "+otherID || picker.visible {
		t.Fatalf("resume selection = %+v, visible=%v", action, picker.visible)
	}
}

func TestSessionPickerEscapeReturnsToConversationAndSanitizesTitles(t *testing.T) {
	picker := newSessionPicker()
	picker.open([]agent.SessionInfo{{ID: strings.Repeat("c", 32), Title: "[fg:red]unsafe\ntitle"}}, "")
	if strings.Contains(picker.list.Rows[0], "[fg:red]") || strings.ContainsRune(picker.list.Rows[0], '\n') {
		t.Fatalf("session title was not sanitized: %q", picker.list.Rows[0])
	}
	picker.handle(ui.Event{ID: "<Escape>"})
	if picker.visible {
		t.Fatal("Escape did not close the session picker")
	}
}

func TestSessionPickerBackKeysAreHandledBeforeGlobalNavigation(t *testing.T) {
	for _, event := range []ui.Event{
		{ID: "<Escape>"},
		{ID: "b"},
		{ID: "B"},
		{ID: "<Backspace>"},
	} {
		workspace := newAgentWorkspace()
		workspace.sessions.open(nil, "")
		if !handleSessionPickerBack(workspace, event) || workspace.sessions.visible {
			t.Errorf("back event %q did not dismiss the picker", event.ID)
		}
	}
}

func TestSessionsCommandOpensPicker(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	session, err := agent.New(agent.Config{ProviderID: llm.OpenAI, Model: "test-model", APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Restore(root); err != nil {
		t.Fatal(err)
	}
	workspace := newAgentWorkspace()
	rows := []string{"conversation stays intact"}
	follow := false
	if !handleSessionCommand(commandSessions, false, session, root, &workspace, &rows, &follow, func(string) {}) {
		t.Fatal("/sessions was not handled")
	}
	if !workspace.sessions.visible || len(workspace.sessions.list.Rows) != 1 || rows[0] != "conversation stays intact" || !follow {
		t.Fatalf("picker did not open cleanly: visible=%v rows=%v transcript=%v follow=%v", workspace.sessions.visible, workspace.sessions.list.Rows, rows, follow)
	}
}
