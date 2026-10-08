package tui

import (
	"strings"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/agent"
)

type sessionPicker struct {
	list     *widgets.List
	sessions []agent.SessionInfo
	visible  bool
}

func newSessionPicker() *sessionPicker {
	list := widgets.NewList()
	list.Border = true
	list.WrapText = false
	themeList(list)
	return &sessionPicker{list: list}
}

func (picker *sessionPicker) open(sessions []agent.SessionInfo, activeID string) {
	picker.sessions = append(picker.sessions[:0], sessions...)
	picker.list.Rows = make([]string, len(picker.sessions))
	picker.list.SelectedRow = 0
	for index, session := range picker.sessions {
		marker := "  "
		if session.ID == activeID {
			marker = "* "
			picker.list.SelectedRow = index
		}
		title := strings.TrimSpace(strings.Map(func(char rune) rune {
			if char < ' ' || char == 0x7f {
				return ' '
			}
			if char == '[' {
				return '('
			}
			if char == ']' {
				return ')'
			}
			return char
		}, session.Title))
		if title == "" {
			title = "Untitled session"
		}
		if titleRunes := []rune(title); len(titleRunes) > 120 {
			title = string(titleRunes[:119]) + "…"
		}
		picker.list.Rows[index] = marker + title + "  ·  " + session.UpdatedAt.Local().Format("2006-01-02 15:04") + "  ·  " + shortSessionID(session.ID)
	}
	if len(picker.list.Rows) == 0 {
		picker.list.Rows = []string{"No saved sessions"}
		picker.list.SelectedRow = -1
	}
	picker.visible = true
}

func (picker *sessionPicker) handle(event ui.Event) workspaceAction {
	event = normalizeKeyEvent(event)
	if event.ID == "<Escape>" || event.ID == "q" || isSecondaryBackKey(event.ID) {
		picker.visible = false
		return workspaceAction{}
	}
	switch event.ID {
	case "<Up>", "k":
		picker.list.ScrollUp()
	case "<Down>", "j":
		picker.list.ScrollDown()
	case "<PageUp>":
		picker.list.ScrollPageUp()
	case "<PageDown>":
		picker.list.ScrollPageDown()
	case "<Enter>":
		index := picker.list.SelectedRow
		if index >= 0 && index < len(picker.sessions) {
			picker.visible = false
			return workspaceAction{prompt: commandResume + " " + picker.sessions[index].ID}
		}
	}
	return workspaceAction{}
}

func handleSessionPickerBack(workspace *agentWorkspace, event ui.Event) bool {
	event = normalizeKeyEvent(event)
	if !workspace.sessions.visible || (event.ID != "<Escape>" && !isSecondaryBackKey(event.ID)) {
		return false
	}
	workspace.sessions.handle(event)
	return true
}

func (picker *sessionPicker) draw(width, height int) {
	picker.list.Title = "Saved sessions · select one to resume"
	picker.list.TitleBottom = "Up/Down move | Enter resume | Esc/b back"
	picker.list.SetRect(0, 0, width, height)
	ui.Render(picker.list)
}
