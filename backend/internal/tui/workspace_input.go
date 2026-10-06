package tui

import (
	ui "github.com/metaspartan/gotui/v5"
	"image"
	"strings"
	"unicode/utf8"
)

func (workspace *agentWorkspace) Handle(event ui.Event, busy bool) workspaceAction {
	if workspace.mentions.visible {
		switch event.ID {
		case "<Escape>":
			workspace.mentions.dismiss(workspace.composer)
			return workspaceAction{}
		case "<Up>":
			workspace.mentions.list.ScrollUp()
			return workspaceAction{}
		case "<Down>":
			workspace.mentions.list.ScrollDown()
			return workspaceAction{}
		case "<Enter>", "<Tab>":
			if workspace.mentions.choose(workspace.composer) {
				return workspaceAction{}
			}
		}
	}
	switch event.ID {
	case "<F2>", "<Escape>":
		return workspaceAction{back: true}
	case "<F3>":
		return workspaceAction{provider: true}
	case "<F4>":
		return workspaceAction{model: true}
	case "<C-o>":
		workspace.details = !workspace.details
		return workspaceAction{}
	case "<C-c>":
		return workspaceAction{quit: true}
	case "<PageUp>":
		workspace.transcript.ScrollPageUp()
		return workspaceAction{scroll: true}
	case "<PageDown>":
		workspace.transcript.ScrollPageDown()
		return workspaceAction{scroll: true, follow: workspace.transcript.AtBottom()}
	}
	switch event.ID {
	case "<Enter>":
		prompt := strings.TrimSpace(workspace.composer.Text)
		if prompt == "" {
			return workspaceAction{}
		}
		workspace.composer.Text = ""
		workspace.composer.Cursor = image.Point{}
		return workspaceAction{prompt: prompt, queued: busy}
	case "<C-j>":
		workspace.composer.InsertNewline()
	case "<Backspace>", "<C-h>":
		workspace.composer.DeleteRune()
	case "<Left>":
		workspace.composer.MoveCursor(-1, 0)
	case "<Right>":
		workspace.composer.MoveCursor(1, 0)
	case "<Up>":
		workspace.composer.MoveCursor(0, -1)
	case "<Down>":
		workspace.composer.MoveCursor(0, 1)
	case "<Home>":
		workspace.composer.Cursor.X = 0
	case "<End>":
		lines := strings.Split(workspace.composer.Text, "\n")
		workspace.composer.Cursor.X = len([]rune(lines[workspace.composer.Cursor.Y]))
	case "<C-a>":
		workspace.composer.Cursor = image.Point{}
	case "<C-e>":
		lines := strings.Split(workspace.composer.Text, "\n")
		workspace.composer.Cursor.Y = len(lines) - 1
		workspace.composer.Cursor.X = len([]rune(lines[workspace.composer.Cursor.Y]))
	case "<Space>":
		workspace.insert(' ')
	default:
		if event.Type == ui.KeyboardEvent {
			for _, char := range event.ID {
				if char == '\n' || char == '\r' || char == '\t' {
					continue
				}
				workspace.insert(char)
			}
		}
	}
	return workspaceAction{}
}

func (workspace *agentWorkspace) insert(char rune) {
	if utf8.RuneCountInString(workspace.composer.Text) < maxPromptRunes {
		workspace.composer.InsertRune(char)
	}
}
