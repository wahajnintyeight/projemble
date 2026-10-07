package tui

import (
	"fmt"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"

	"projemble/internal/agent"
	"projemble/internal/projectstore"
)

func accessModeChoices() []catalogChoice {
	return []catalogChoice{
		{name: "Read-only", description: "Read project files; changes and commands are blocked"},
		{name: "Full access", description: "Read and edit freely; arbitrary shell commands require approval"},
		{name: "Ask always", description: "Approve each file read, edit, or command before it runs"},
	}
}

func saveAgentAccessMode(config *projectstore.Config, mode agent.AccessMode) error {
	if !mode.Valid() {
		return fmt.Errorf("unknown agent access mode %q", mode)
	}
	next := *config
	next.AgentAccessMode = string(mode)
	if err := projectstore.SaveDefault(next); err != nil {
		return err
	}
	*config = next
	return nil
}

func replyToPermission(update *generationUpdate, allow bool) {
	if update == nil || update.decision == nil {
		return
	}
	select {
	case update.decision <- allow:
	default:
	}
}

func accessModeIndex(value string) int {
	switch agent.AccessMode(value) {
	case agent.AccessReadOnly:
		return 0
	case agent.AccessFull:
		return 1
	default:
		return 2
	}
}

func accessModeAt(index int) agent.AccessMode {
	switch index {
	case 0:
		return agent.AccessReadOnly
	case 1:
		return agent.AccessFull
	default:
		return agent.AccessAskAlways
	}
}

func accessModeLabel(value string) string {
	switch agent.AccessMode(value) {
	case agent.AccessReadOnly:
		return "Read-only"
	case agent.AccessFull:
		return "Full access"
	default:
		return "Ask always"
	}
}

func renderPermissionPrompt(list *widgets.List, request agent.PermissionRequest, selected, width, height int) {
	updateChoiceList(list, "Approve agent action", []catalogChoice{
		{name: "Allow once", description: "Permit only this requested action"},
		{name: "Deny", description: "Keep the project unchanged by this action"},
	}, selected, width, height)
	intro := fmt.Sprintf("The agent wants to %s.\n\nTarget: %s\n\nAllow this action once? Ask always will prompt again before the next action.", request.Action, request.Target)
	if height < 12 || width < 30 {
		ui.Render(list)
		return
	}
	banner := onboardingBanner("Projemble / Permission request", intro, width, height)
	list.SetRect(0, banner.Max.Y, width, height)
	setFooter(&list.Block, "Enter selected action | Esc deny (Deny selected by default)", false)
	ui.Render(banner, list)
}
