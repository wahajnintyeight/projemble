package tui

import (
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
)

// Use ANSI-friendly semantic colors and let the terminal keep its own canvas.
const (
	colorAccent   = "cyan"
	colorThinking = "violet"
	colorRead     = "lightblue"
	colorAdded    = "green"
	colorChanged  = "gold"
	colorRemoved  = "tomato"
	colorCheck    = "lightblue"
	colorError    = "red"
	colorUser     = "cyan"
)

func activateTUITheme() func() {
	previous := ui.Theme
	themed := previous
	themed.Default = ui.NewStyle(ui.ColorClear)
	themed.Block.Title = ui.NewStyle(ui.ColorLightCyan, ui.ColorClear, ui.ModifierBold)
	themed.Block.Border = ui.NewStyle(ui.ColorDarkCyan)
	themed.Paragraph.Text = ui.NewStyle(ui.ColorClear)
	themed.List.Text = ui.NewStyle(ui.ColorClear)
	themed.Tree.Text = ui.NewStyle(ui.ColorClear)
	themed.Tab.Active = focusedStyle()
	ui.Theme = themed
	return func() { ui.Theme = previous }
}

func themeList(list *widgets.List) {
	list.TextStyle = ui.NewStyle(ui.ColorClear)
	list.SelectedStyle = focusedStyle()
}

func themeInput(input *widgets.Input) {
	input.TextStyle = ui.NewStyle(ui.ColorClear)
	input.CursorStyle = focusedStyle()
}

func focusedStyle() ui.Style {
	return ui.NewStyle(ui.ColorClear, ui.ColorClear, ui.ModifierReverse|ui.ModifierBold)
}

func styleLabel(label string) string {
	return "[" + label + "](fg:" + colorAccent + ",mod:bold)"
}

func styleError(message string) string {
	return "[!](fg:" + colorError + ",mod:bold) " + message
}

func setFooter(block *ui.Block, text string, isError bool) {
	if !isError {
		text += " · Ctrl+L redraw"
	}
	block.TitleBottom = text
	block.TitleBottomStyle = ui.Theme.Block.Title
	if isError {
		block.TitleBottomStyle = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierBold)
	}
}
