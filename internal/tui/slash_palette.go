package tui

import (
	"image"
	"strconv"
	"strings"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
)

type slashOption struct {
	command     string
	usage       string
	description string
}

var slashOptions = []slashOption{
	{command: commandCompact, usage: commandCompact + " [focus]", description: "Summarize this session"},
	{command: commandNew, usage: commandNew + " [carry]", description: "Start a fresh session"},
	{command: commandSessions, usage: commandSessions, description: "Choose a saved session"},
	{command: commandResume, usage: commandResume + " <id>", description: "Resume a saved session"},
}

type slashPalette struct {
	list      *widgets.List
	query     string
	selected  int
	matches   []slashOption
	dismissed string
	visible   bool
}

func newSlashPalette() *slashPalette {
	list := widgets.NewList()
	list.Border = true
	themeList(list)
	return &slashPalette{list: list}
}

func (palette *slashPalette) active(composer *messageComposer) bool {
	query, signature, ok := slashTokenAtCursor(composer)
	if !ok || signature == palette.dismissed {
		palette.visible = false
		return false
	}
	previous := ""
	if palette.selected >= 0 && palette.selected < len(palette.matches) {
		previous = palette.matches[palette.selected].command
	}
	palette.query = strings.ToLower(query)
	palette.matches = palette.matches[:0]
	for _, option := range slashOptions {
		if strings.Contains(option.command, palette.query) {
			palette.matches = append(palette.matches, option)
		}
	}
	if len(palette.matches) == 0 {
		palette.visible = false
		return false
	}
	palette.selected = 0
	for index, option := range palette.matches {
		if option.command == previous {
			palette.selected = index
			break
		}
	}
	palette.visible = true
	return true
}

func slashTokenAtCursor(composer *messageComposer) (query, signature string, ok bool) {
	if composer.Cursor.Y != 0 || strings.ContainsRune(composer.Text, '\n') {
		return "", "", false
	}
	runes := []rune(composer.Text)
	if len(runes) == 0 || runes[0] != '/' || composer.Cursor.X < 1 || composer.Cursor.X > len(runes) {
		return "", "", false
	}
	end := len(runes)
	for index, char := range runes[1:] {
		if char == ' ' || char == '\t' {
			end = index + 1
			break
		}
	}
	if composer.Cursor.X > end {
		return "", "", false
	}
	query = string(runes[1:composer.Cursor.X])
	signature = composer.Text + "\x00" + strconv.Itoa(composer.Cursor.X)
	return query, signature, true
}

func (palette *slashPalette) handle(event ui.Event, composer *messageComposer) bool {
	if !palette.active(composer) {
		return false
	}
	_, signature, _ := slashTokenAtCursor(composer)
	switch event.ID {
	case "<Escape>":
		palette.dismissed = signature
		palette.visible = false
		return true
	case "<Up>":
		palette.selected = (palette.selected + len(palette.matches) - 1) % len(palette.matches)
		return true
	case "<Down>":
		palette.selected = (palette.selected + 1) % len(palette.matches)
		return true
	case "<Enter>", "<Tab>":
		palette.complete(composer)
		return true
	default:
		return false
	}
}

func (palette *slashPalette) complete(composer *messageComposer) {
	_, _, ok := slashTokenAtCursor(composer)
	if !ok || palette.selected < 0 || palette.selected >= len(palette.matches) {
		return
	}
	runes := []rune(composer.Text)
	end := len(runes)
	for index, char := range runes[1:] {
		if char == ' ' || char == '\t' {
			end = index + 1
			break
		}
	}
	completed := []rune(palette.matches[palette.selected].command)
	if end == len(runes) {
		completed = append(completed, ' ')
	}
	runes = append(completed, runes[end:]...)
	composer.Text = string(runes)
	composer.Cursor = image.Pt(len(completed), 0)
	palette.visible = false
	palette.dismissed = ""
}

func (palette *slashPalette) draw(width, end, available int) int {
	if !palette.visible || len(palette.matches) == 0 || available < 3 {
		return 0
	}
	palette.list.Title = "Commands"
	palette.list.TitleBottom = "↑/↓ move · Enter or Tab complete · Esc close"
	palette.list.Rows = make([]string, len(palette.matches))
	for index, option := range palette.matches {
		palette.list.Rows[index] = option.usage + "  " + option.description
	}
	palette.list.SelectedRow = palette.selected
	rows := min(len(palette.matches), max(1, min(5, available-2)))
	height := rows + 2
	palette.list.SetRect(0, end-height, width, end)
	ui.Render(palette.list)
	return height
}
