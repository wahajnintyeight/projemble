package tui

import (
	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/agent"
	"projemble/internal/projectstore"
)

func openAccessMode(current, returnTo *page, selected *int, options generationOptions, list *widgets.List) {
	*returnTo = *current
	*selected = accessModeIndex(options.AccessMode)
	list.SelectedRow = *selected
	*current = accessModePage
}

func handleAccessModeInput(id string, current *page, returnTo page, list *widgets.List, config *projectstore.Config, options *generationOptions, session *agent.Agent, rows *[]string, pending **generationUpdate) (bool, string) {
	switch *current {
	case accessModePage:
		switch id {
		case "<Enter>":
			mode := accessModeAt(list.SelectedRow)
			if err := saveAgentAccessMode(config, mode); err != nil {
				return true, "Could not save access mode: " + err.Error()
			}
			options.AccessMode = string(mode)
			if session != nil {
				if err := session.SetAccessMode(mode); err != nil {
					return true, err.Error()
				}
			}
			if returnTo == agentProgressPage {
				*rows = appendActivity(*rows, "Agent access mode set to "+accessModeLabel(string(mode))+".")
			}
			*current = returnTo
			return true, ""
		case "<Escape>", "b", "<Backspace>":
			*current = returnTo
			return true, ""
		}
	case approvalPage:
		switch id {
		case "<Enter>":
			replyToPermission(*pending, list.SelectedRow == 0)
			*pending = nil
			*current = agentProgressPage
			return true, ""
		case "<Escape>", "b", "<Backspace>":
			replyToPermission(*pending, false)
			*pending = nil
			*current = agentProgressPage
			return true, ""
		}
	}
	return false, ""
}
