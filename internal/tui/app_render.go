package tui

import (
	"fmt"
	"strings"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

type wizardView struct {
	currentPage                                                                                          page
	list                                                                                                 *widgets.List
	models                                                                                               *modelPicker
	nameInput, descriptionInput, locationInput, keyInput, modelInput                                     *widgets.Input
	selectedMode, selectedProvider, selectedShape, selectedArchitecture, selectedWorkload, selectedStack int
	providerConfig                                                                                       generationOptions
	validationMessage, saveError, projectPath, configPath                                                string
	name, description                                                                                    string
	authPending                                                                                          bool
	reopening                                                                                            *projectstore.Project
	selection                                                                                            *projectWizard
}

func renderWizardPage(view wizardView, width, height int) (bool, error) {
	list := view.list
	switch view.currentPage {
	case generationModePage:
		updateChoiceList(list, "Generation mode", generationModeChoices(), view.selectedMode, width, height)
		setFooter(&list.Block, "Loaded default: "+generationLabel(view.providerConfig)+" | Up/Down or j/k  Select  Enter  Continue  b  Back  q  Quit", false)
		renderOnboardingChoices(list, blueprintIntroduction(view.selectedShape, view.selectedArchitecture), width, height)
	case providerPage:
		renderProviderPage(list, view.reopening, view.selectedProvider, view.authPending, view.validationMessage, width, height)
	case apiKeyPage:
		setInputLayout(view.keyInput, width, height)
		setInputFooter(view.keyInput, view.validationMessage, "Enter save | Esc or Ctrl+B back")
		renderOnboardingInput(view.keyInput, view.currentPage, width, height)
	case aiModelPage:
		view.models.prepare(view.providerConfig)
		view.models.render(view.modelInput, width, height, view.validationMessage)
	case projectNamePage, projectDescriptionPage, projectLocationPage:
		input := view.nameInput
		if view.currentPage == projectDescriptionPage {
			input = view.descriptionInput
		} else if view.currentPage == projectLocationPage {
			input = view.locationInput
		}
		setInputLayout(input, width, height)
		setInputFooter(input, view.validationMessage, "Enter continue | Esc back")
		renderOnboardingInput(input, view.currentPage, width, height)
	case repairPathPage:
		view.locationInput.Title = "Existing project directory"
		view.locationInput.Placeholder = "Enter the current absolute project folder path"
		setInputLayout(view.locationInput, width, height)
		setInputFooter(view.locationInput, view.validationMessage, "Enter update saved path | Esc cancel")
		ui.Render(view.locationInput)
	case workloadPage:
		updateChoiceList(list, "What are you building?", workloadChoices(), view.selectedWorkload, width, height)
		renderOnboardingChoices(list, "Choose how this Go project runs. The workload sets the entry point; patterns and capabilities can be added independently.", width, height)
	case patternPage:
		updateChoiceList(list, "Application patterns · Space toggles · optional", patternChoices(view.selection.patterns), list.SelectedRow, width, height)
		setFooter(&list.Block, "Space toggle | Enter continue | Esc/b back | no selection is fine", false)
		renderOnboardingChoices(list, "Combine patterns as needed. Standard backend, RAG, agent, and chatbot are separate from how the project runs.", width, height)
	case topologyPage:
		updateChoiceList(list, "Service topology", topologyChoices(), topologyIndex(view.selectedShape), width, height)
		renderOnboardingChoices(list, "Choose one deployable service or multiple services. Non-HTTP workloads do not need a service topology.", width, height)
	case appShapePage:
		updateChoiceList(list, "Choose the application shape", appShapeChoices(), view.selectedShape, width, height)
		renderOnboardingChoices(list, shapeIntroduction(), width, height)
	case architecturePage:
		updateChoiceList(list, "Code architecture", architectureChoices(appShapeIDAt(view.selectedShape)), view.selectedArchitecture, width, height)
		renderOnboardingChoices(list, architectureIntroduction(view.selectedShape), width, height)
	case stackPage:
		updateChoiceList(list, "Implementation stack", stackChoices(), view.selectedStack, width, height)
		setFooter(&list.Block, "Enter continue | planned stacks cannot be selected | Esc/b back", false)
		renderOnboardingChoices(list, "Go is the only implemented stack in this rollout. Node.js, NestJS, Laravel, and PHP are shown as planned until their generators are verified.", width, height)
	case capabilityPage:
		updateChoiceList(list, "Optional capabilities · Space toggles", capabilityChoices(view.selection.capabilities), list.SelectedRow, width, height)
		setFooter(&list.Block, "Space toggle supported item | Enter continue | planned items are disabled | Esc/b back", false)
		renderOnboardingChoices(list, "Optional integrations are composable. Choose at most one database; skip the step to generate a minimal scaffold.", width, height)
	case editOptionsPage:
		updateChoiceList(list, "Adjust your blueprint", editBlueprintChoices(workloadIDAt(view.selectedWorkload)), list.SelectedRow, width, height)
		setFooter(&list.Block, "Enter edit | Esc/b review", false)
		renderOnboardingChoices(list, "Everything here is optional to revisit. Choose one area to adjust, or press Esc or b to return to your project review.", width, height)
	case summaryPage:
		return true, renderSummaryPage(list, view.selectedShape, view.selectedArchitecture, view.name, view.description, view.projectPath, view.providerConfig, view.selection, "Enter generate project", view.saveError, width, height)
	case savedPage:
		return true, renderSavedPage(list, view.selectedShape, view.selectedArchitecture, view.name, view.description, view.projectPath, view.configPath, view.providerConfig, view.selection, width, height)
	case projectManagePage:
		renderProjectManagementPage(list, view.reopening, view.validationMessage, width, height)
		return true, nil
	case projectDeletePage:
		renderProjectDeletePage(list, view.reopening, view.validationMessage, width, height)
		return true, nil
	default:
		return false, nil
	}
	return true, nil
}

func setInputFooter(input *widgets.Input, validation, fallback string) {
	if validation != "" {
		setFooter(&input.Block, validation, true)
	} else {
		setFooter(&input.Block, fallback, false)
	}
}

func workloadIndex(id string) int {
	for index, workload := range catalog.Workloads() {
		if workload.ID == id {
			return index
		}
	}
	return 0
}

func topologyIndex(shapeIndex int) int {
	shape := appShapeIDAt(shapeIndex)
	for index, topology := range catalog.Topologies() {
		if topology.ID == shape {
			return index
		}
	}
	return 0
}

func renderHomePage(list *widgets.List, config projectstore.Config, selected int, validation string, width, height int) {
	updateChoiceList(list, "Projemble | Projects", homeChoices(config), selected, width, height)
	setFooter(&list.Block, "Enter open | p repair path | n new project | r provider | F6 agent access | Esc/q quit", false)
	if validation != "" {
		setFooter(&list.Block, validation, true)
	}
	renderProjectHome(list, "Select a saved project to continue, or create a new blueprint.", width, height)
}

func renderProviderPage(list *widgets.List, reopening *projectstore.Project, selected int, authPending bool, validation string, width, height int) {
	updateChoiceList(list, "Choose an AI provider", providerChoices(), selected, width, height)
	if reopening != nil {
		list.Title = "Provider for " + reopening.Name
	}
	if authPending {
		setFooter(&list.Block, "Finish sign-in in your browser | Esc/b cancels | q quits", false)
	} else if validation != "" {
		setFooter(&list.Block, validation+" | Enter retry | Esc/b back", true)
	}
	renderOnboardingChoices(list, "Agent / Provider connection\nChoose the provider that will implement features on your generated scaffold. Saved credentials can be reused across projects.", width, height)
}

func renderSummaryPage(list *widgets.List, selectedShape, selectedArchitecture int, name, description, projectPath string, options generationOptions, selection *projectWizard, action, saveError string, width, height int) error {
	template, ok := templateForChoices(selectedShape, selectedArchitecture)
	if !ok {
		return fmt.Errorf("selected shape and architecture have no matching template")
	}
	updateSummary(list, name, description, template, projectPath, options, action, saveError, width, height)
	patterns := make([]string, 0, len(selection.patterns))
	for _, id := range selection.patterns {
		if item, ok := catalog.PatternByID(id); ok {
			patterns = append(patterns, item.Name)
		}
	}
	list.Rows = append(list.Rows, "Workload:       "+template.WorkloadID, "Patterns:       "+summaryList(patterns), "Capabilities:   "+summaryCapabilities(selection.capabilities))
	ui.Render(list)
	return nil
}

func renderSavedPage(list *widgets.List, selectedShape, selectedArchitecture int, name, description, projectPath, configPath string, options generationOptions, selection *projectWizard, width, height int) error {
	if err := renderSummaryPage(list, selectedShape, selectedArchitecture, name, description, projectPath, options, selection, "", "", width, height); err != nil {
		return err
	}
	list.Title = "Saved project"
	list.Rows[10] = "Saved project profile. Your source files remain in the project directory."
	list.Rows = append(list.Rows,
		"",
		"Config: "+configPath,
		"Starter source files were generated.",
		"Press Esc or b to return to projects.",
	)
	setFooter(&list.Block, "Esc/b projects | q exit", false)
	ui.Render(list)
	return nil
}

func summaryList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func summaryCapabilities(ids []string) string {
	var names []string
	for _, id := range ids {
		if capability, ok := catalog.CapabilityByID(id); ok {
			names = append(names, capability.Name)
		}
	}
	return summaryList(names)
}
