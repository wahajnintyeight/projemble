package tui

import (
	"fmt"
	ui "github.com/metaspartan/gotui/v5"
	"projemble/internal/llm"
)

// Wide terminals use a session rail; narrow terminals retain a stacked layout.
func (w *agentWorkspace) Render(width, height int, rows []string, options generationOptions, path string, running, follow bool, queued int) {
	provider, _ := providerAtIndex(options.Provider)
	w.header.Title = "Session"
	w.header.Text = w.sidebarText(provider, options.Model, path, running, queued)
	if options.Provider == llm.OpenAIWeb {
		w.header.Text = w.sidebarText(provider, options.Model, path, running, queued) + "\n\n" + styleLabel("Thinking effort:") + " " + reasoningEffortLabel(options.ReasoningEffort)
	}
	w.header.Text += "\n\n" + styleLabel("Agent access:") + " " + accessModeLabel(options.AccessMode)
	w.navigation.Text = "F2 / Esc  Projects\nF3  Provider\nF4  Model\nF6  Access mode\nCtrl+O  Activity details\nCtrl+B  Hide this panel\nSwitching stops current work."
	if options.Provider == llm.OpenAIWeb {
		w.navigation.Text = "F2 / Esc  Projects\nF3  Provider\nF4  Model\nF5  Thinking effort\nF6  Access mode\nCtrl+O  Activity details\nCtrl+B  Hide this panel\nSwitching stops current work."
	}
	leftWidth, top := width, 0
	wide := width >= 110 && height >= 20
	if w.showSidebar && wide {
		rail := min(42, max(32, width/4))
		leftWidth = width - rail
		navHeight := 7
		w.header.SetRect(leftWidth, 0, width, height-navHeight)
		w.navigation.SetRect(leftWidth, height-navHeight, width, height)
	} else if w.showSidebar {
		top = min(max(0, height-1), min(9, max(3, height/3)))
		w.header.Title = fmt.Sprintf("Projemble · %s · %s", provider, options.Model)
		w.header.Text = w.statusText(provider, path, running, queued)
		w.header.SetRect(0, 0, width, top)
	}
	composerWidth := max(1, leftWidth-2)
	composerHeight := min(min(14, max(6, height/2)), max(4, w.composer.visualLines(composerWidth)+2))
	if top > 0 {
		composerHeight = min(composerHeight, min(height-top, max(3, height-top-4)))
	}
	composerTop := height - composerHeight
	popupHeight := 0
	if w.mentions.active(path, w.composer) {
		popupHeight = min(10, max(0, composerTop-top))
		if popupHeight < 3 {
			popupHeight = 0
		}
	}
	transcriptBottom := max(top, composerTop-popupHeight)
	w.transcript.SetRect(0, top, leftWidth, transcriptBottom)
	w.transcript.content(rows, w.details, follow)
	w.composer.SetRect(0, composerTop, leftWidth, height)
	w.composer.TitleBottom = "Enter send · F2 projects · F3 provider · F4 model · F6 access · Ctrl+B panel · Ctrl+L redraw"
	if options.Provider == llm.OpenAIWeb {
		w.composer.TitleBottom = "Enter send · F2 projects · F3 provider · F4 model · F5 thinking · F6 access · Ctrl+B panel · Ctrl+L redraw"
	}
	if running {
		w.composer.TitleBottom = fmt.Sprintf("%c Working · queued %d/%d · Enter queue · Ctrl+B panel · Ctrl+L redraw", agentSpinnerFrames[w.spinner%len(agentSpinnerFrames)], queued, maxPendingPrompts)
	}
	if running && options.Provider == llm.OpenAIWeb {
		w.composer.TitleBottom = fmt.Sprintf("%c Working · queued %d/%d · Enter queue · F5 thinking · Ctrl+B panel", agentSpinnerFrames[w.spinner%len(agentSpinnerFrames)], queued, maxPendingPrompts)
	}
	w.composer.ShowCursor = true
	if !w.showSidebar {
		ui.Render(w.transcript, w.composer)
	} else if wide {
		ui.Render(w.header, w.navigation, w.transcript, w.composer)
	} else {
		ui.Render(w.header, w.transcript, w.composer)
	}
	if popupHeight > 0 {
		w.mentions.draw(path, w.composer, leftWidth, composerTop, popupHeight)
	}
}
