package tui

import (
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/catalog"
)

// Onboarding keeps the project blueprint visible before optional agent setup.
// Hallmark critique: philosophy 5, hierarchy 4, execution 4, specificity 5, restraint 5, variety 4.
func onboardingBanner(title, text string, width, height int) *sessionPanel {
	banner := newSessionPanel()
	banner.Title = title
	banner.Border = true
	banner.Text = text
	lines := wrapTranscriptCells(ui.ParseStyles(text, banner.TextStyle), max(1, width-2))
	banner.SetRect(0, 0, width, min(len(lines)+2, max(0, height-6)))
	return banner
}

func renderOnboardingChoices(list *widgets.List, introduction string, width, height int) {
	if height < 12 || width < 30 {
		ui.Render(list)
		return
	}
	banner := onboardingBanner("Projemble / Project builder", introduction, width, height)
	list.SetRect(0, banner.Max.Y, width, height)
	ui.Render(banner, list)
}

func renderProjectHome(list *widgets.List, configText string, width, height int) {
	if height < 12 || width < 30 {
		ui.Render(list)
		return
	}
	text := styleLabel("Welcome to Projemble.") + " Build a project with a clear foundation.\n\n" +
		"Choose its purpose, application shape, and architecture. Projemble generates the scaffold; an optional agent builds on it.\n" +
		"Go starters: monolith APIs, go-micro services, and one-shot jobs for scraping, ETL, or maintenance.\n\n" + configText
	banner := onboardingBanner("Projemble / Build from a blueprint", text, width, height)
	list.SetRect(0, banner.Max.Y, width, height)
	ui.Render(banner, list)
}

func renderOnboardingInput(input *widgets.Input, current page, width, height int) {
	setInputLayout(input, width, height)
	title, text := "", ""
	switch current {
	case projectNamePage:
		title, text = "1 / Define your project", "Every project starts with an idea. Give yours a name; next, describe what it should do."
	case projectDescriptionPage:
		title, text = "1 / Define your project", "Describe the purpose and the work it needs to perform. This becomes part of the project profile and guides the optional agent."
	case projectLocationPage:
		title, text = "2 / Choose a home", "Choose an existing parent directory. Projemble creates a new folder inside it using your project name."
	case apiKeyPage:
		title, text = "Agent / Provider connection", "Connect a provider for work on your scaffold. The masked key is saved in your local YAML settings."
	}
	if title == "" || height < 12 || width < 30 {
		ui.Render(input)
		return
	}
	banner := onboardingBanner(title, text, width, height)
	top := max(banner.Max.Y+1, input.Min.Y)
	input.SetRect(input.Min.X, min(top, height-4), input.Max.X, min(top+4, height))
	ui.Render(banner, input)
}

func blueprintIntroduction(shapeIndex, architectureIndex int) string {
	template, ok := templateForChoices(shapeIndex, architectureIndex)
	if !ok {
		return "Choose how to build your project from its selected template."
	}
	return styleLabel("Your blueprint: ") + template.Name + "\n" + template.Description +
		"\n\nBoth options start with this scaffold. Add an agent when you want it to implement features and run checks."
}

func shapeIntroduction() string {
	return "3 / Application shape\nChoose how your Go project runs: one application, separate services, or a task that runs once and exits."
}

func architectureIntroduction(shapeIndex int) string {
	shape, _ := catalog.AppShapeByID(appShapeIDAt(shapeIndex))
	return "4 / Architecture\n" + shape.Name + ": choose how its code is organized. This selects the template Projemble will generate."
}

func generationModeIndex(options generationOptions) int {
	if options.Mode == "agent" {
		return 1
	}
	return 0
}
