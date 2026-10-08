package tui

import (
	"fmt"
	ui "github.com/metaspartan/gotui/v5"
	"projemble/internal/llm"
	"strings"
)

// Wide terminals use a session rail; narrow terminals retain a stacked layout.
func (w *agentWorkspace) Render(width, height int, rows []string, options generationOptions, path string, running, follow bool, queued int) {
	if w.sessions.visible {
		w.sessions.draw(width, height)
		return
	}
	provider, _ := providerAtIndex(options.Provider)
	w.header.Title = "Session"
	w.header.Text = w.sidebarText(provider, options.Model, path, running, queued)
	if options.Provider == llm.OpenAIWeb {
		w.header.Text = w.sidebarText(provider, options.Model, path, running, queued) + "\n\n" + styleLabel("Thinking effort:") + " " + reasoningEffortLabel(options.ReasoningEffort)
	}
	w.header.Text += "\n\n" + styleLabel("Agent access:") + " " + accessModeLabel(options.AccessMode)
	w.navigation.Text = "F2 / Esc  Projects\nF3  Provider\nF4  Model\nF6  Access mode\n/  Command menu\nCtrl+O  Activity details\nCtrl+B  Hide this panel"
	if options.Provider == llm.OpenAIWeb {
		w.navigation.Text = "F2 / Esc  Projects\nF3  Provider\nF4  Model\nF5  Thinking effort\nF6  Access mode\n/  Command menu\nCtrl+O  Activity details\nCtrl+B  Hide this panel"
	}
	leftWidth, top := width, 0
	wide := width >= 110 && height >= 20
	if w.showSidebar && wide {
		rail := min(42, max(32, width/4))
		leftWidth = width - rail
		navHeight := min(height/2, max(7, len(strings.Split(w.navigation.Text, "\n"))+2))
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
	showMentions := w.mentions.active(path, w.composer)
	showCommands := w.commands.active(w.composer)
	if showMentions {
		popupHeight = min(10, max(0, composerTop-top))
		if popupHeight < 3 {
			popupHeight = 0
		}
	} else if showCommands {
		popupHeight = min(7, max(0, composerTop-top))
		if popupHeight < 3 {
			popupHeight = 0
		}
	}
	transcriptBottom := max(top, composerTop-popupHeight)
	w.transcript.SetRect(0, top, leftWidth, transcriptBottom)
	w.transcript.content(rows, w.details, follow)
	w.composer.SetRect(0, composerTop, leftWidth, height)
	if w.showSidebar && wide {
		w.composer.TitleBottom = "Enter send · / commands · F2 projects · Ctrl+B panel"
	} else {
		w.composer.TitleBottom = "Enter send · / commands · F2 projects · F3 provider · F4 model · F6 access"
		if options.Provider == llm.OpenAIWeb {
			w.composer.TitleBottom = "Enter · / commands · F2 projects · F3 provider · F4 model · F5 thinking · F6 access"
		}
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
		if showMentions {
			w.mentions.draw(path, w.composer, leftWidth, composerTop, popupHeight)
		} else if showCommands {
			w.commands.draw(leftWidth, composerTop, popupHeight)
		}
	}
}
