package tui

import (
	"fmt"
	ui "github.com/metaspartan/gotui/v5"
	"strings"
)

// Wide terminals use a session rail; narrow terminals retain a stacked layout.
func (w *agentWorkspace) Render(width, height int, rows []string, options generationOptions, path string, running, follow bool, queued int) {
	provider, _ := providerAtIndex(options.Provider)
	w.header.Title = "Session"
	w.header.Text = w.sidebarText(provider, options.Model, path, running, queued)
	w.navigation.Text = "F2 / Esc  Projects\nF3  Provider\nF4  Model\nCtrl+O  Activity details\nSwitching stops current work."
	composerHeight := min(6, max(4, strings.Count(w.composer.Text, "\n")+3))
	leftWidth, top := width, 0
	if width >= 110 && height >= 20 {
		rail := min(42, max(32, width/4))
		leftWidth = width - rail
		navHeight := 7
		w.header.SetRect(leftWidth, 0, width, height-navHeight)
		w.navigation.SetRect(leftWidth, height-navHeight, width, height)
	} else {
		top = min(9, max(3, height/3))
		w.header.Title = fmt.Sprintf("Projemble · %s · %s", provider, options.Model)
		w.header.Text = w.statusText(path, running, queued)
		w.header.SetRect(0, 0, width, top)
	}
	bottom := max(top, height-composerHeight)
	w.transcript.SetRect(0, top, leftWidth, bottom)
	w.transcript.content(rows, w.details, follow)
	w.composer.SetRect(0, bottom, leftWidth, height)
	w.composer.TitleBottom = "Enter send · F2 projects · F3 provider · F4 model"
	if running {
		w.composer.TitleBottom = fmt.Sprintf("%c Working · queued %d/%d · Enter queue · F2 projects", agentSpinnerFrames[w.spinner%len(agentSpinnerFrames)], queued, maxPendingPrompts)
	}
	w.composer.ShowCursor = true
	if top == 0 {
		ui.Render(w.header, w.navigation, w.transcript, w.composer)
	} else {
		ui.Render(w.header, w.transcript, w.composer)
	}
}
