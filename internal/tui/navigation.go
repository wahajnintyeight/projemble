package tui

import (
	ui "github.com/metaspartan/gotui/v5"
	"projemble/internal/llm"
	"projemble/internal/projectstore"
)

type page int

const (
	generationModePage page = iota
	providerPage
	apiKeyPage
	aiModelPage
	projectNamePage
	projectDescriptionPage
	projectLocationPage
	workloadPage
	patternPage
	topologyPage
	appShapePage
	architecturePage
	stackPage
	capabilityPage
	summaryPage
	agentProgressPage
	savedPage
	homePage
	repairPathPage
	accessModePage
	approvalPage
)

const (
	maxProjectNameRunes = 60
	maxDescriptionRunes = 160
	maxProjectPathRunes = 4096
	maxAPIKeyRunes      = 4096
	maxModelRunes       = 128
)

// Keep Ctrl+B available to toggle the workspace sidebar.
func normalizeEscape(event ui.Event) ui.Event {
	if event.ID != "<C-b>" && isEscapeKey(event.ID) {
		event.ID = "<Escape>"
	}
	return event
}

// The create row edits defaults; a saved row edits that project's connection.
func homeProviderSelection(config projectstore.Config, selected int) (generationOptions, *projectstore.Project) {
	options := initialGenerationOptions(config)
	options.Mode = "agent"
	if selected <= 0 || selected > len(config.Projects) {
		return options, nil
	}
	project := config.Projects[selected-1]
	if project.GenerationMode == "agent" {
		options = generationOptions{Mode: "agent", AccessMode: options.AccessMode, Provider: llm.ProviderID(project.AIProvider), Model: project.AIModel, ReasoningEffort: llm.ReasoningEffort(project.ReasoningEffort)}
		options.APIKey = rememberedKey(options.Provider, config)
	}
	return options, &project
}

// Save the provider/model pair together, committing in-memory state only on success.
func saveGenerationSelection(config *projectstore.Config, options generationOptions, project *projectstore.Project) error {
	next := *config
	next.Projects = append([]projectstore.Project(nil), config.Projects...)
	next.Generation = projectstore.GenerationDefaults{Mode: "agent", Provider: string(options.Provider), Model: options.Model, ReasoningEffort: string(options.ReasoningEffort)}
	if options.AccessMode != "" {
		next.AgentAccessMode = options.AccessMode
	}
	if project != nil {
		updated := *project
		updated.GenerationMode, updated.AIProvider, updated.AIModel, updated.ReasoningEffort = "agent", string(options.Provider), options.Model, string(options.ReasoningEffort)
		if err := next.Upsert(updated); err != nil {
			return err
		}
	}
	if err := projectstore.SaveDefault(next); err != nil {
		return err
	}
	*config = next
	return nil
}
