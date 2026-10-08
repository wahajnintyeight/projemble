package tui

import (
	"github.com/gdamore/tcell/v3"
	ui "github.com/metaspartan/gotui/v5"
	"projemble/internal/catalog"
	"projemble/internal/llm"
	"projemble/internal/projectstore"
)

type page int

func isTextInputPage(current page) bool {
	switch current {
	case apiKeyPage, aiModelPage, projectNamePage, projectDescriptionPage, projectLocationPage, repairPathPage:
		return true
	default:
		return false
	}
}

func isSecondaryBackKey(id string) bool { return id == "b" || id == "B" || id == "<Backspace>" }

func isBackNavigationKey(id string, current page) bool {
	return id == "<Escape>" || (isSecondaryBackKey(id) && !isTextInputPage(current) && current != agentProgressPage && current != homePage)
}

type pageHistory struct {
	current page
	stack   []page
}

func newPageHistory(initial page) *pageHistory { return &pageHistory{current: initial} }

func (h *pageHistory) Observe(next page) {
	if next != h.current {
		if next == summaryPage {
			for _, previous := range h.stack {
				if previous == summaryPage {
					h.ReturnTo(summaryPage)
					return
				}
			}
		}
		h.stack = append(h.stack, h.current)
		h.current = next
	}
}

func (h *pageHistory) Back() (page, bool) {
	if len(h.stack) == 0 {
		return h.current, false
	}
	last := len(h.stack) - 1
	h.current = h.stack[last]
	h.stack = h.stack[:last]
	return h.current, true
}

func (h *pageHistory) BackTo(current *page) bool {
	target, ok := h.Back()
	if ok {
		*current = target
	}
	return ok
}

func (h *pageHistory) ReturnTo(target page) {
	if h.current == target {
		return
	}
	found := false
	for i := len(h.stack) - 1; i >= 0; i-- {
		if h.stack[i] == target {
			h.stack = h.stack[:i]
			found = true
			break
		}
	}
	if !found {
		h.stack = nil
	}
	h.current = target
}

func (h *pageHistory) Reset(target page) {
	h.current = target
	h.stack = nil
}

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
	editOptionsPage
	summaryPage
	agentProgressPage
	savedPage
	homePage
	repairPathPage
	accessModePage
	approvalPage
	projectManagePage
	projectDeletePage
)

const (
	maxProjectNameRunes = 60
	maxDescriptionRunes = 160
	maxProjectPathRunes = 4096
	maxAPIKeyRunes      = 4096
	maxModelRunes       = 128
)

// Canonicalize key IDs while keeping Ctrl+B available for the sidebar toggle.
func normalizeKeyEvent(event ui.Event) ui.Event {
	if key, ok := event.Payload.(*tcell.EventKey); ok && key.Key() == tcell.KeyEsc {
		event.ID = "<Escape>"
	}
	if event.ID == " " {
		event.ID = "<Space>"
	}
	if event.ID != "<C-b>" && isEscapeKey(event.ID) {
		event.ID = "<Escape>"
	}
	return event
}

func blueprintEditPageAt(selected int, workload string) page {
	if selected >= 0 && selected < 4 {
		return [...]page{projectNamePage, projectDescriptionPage, projectLocationPage, workloadPage}[selected]
	}
	pages := []page{architecturePage, capabilityPage, generationModePage, providerPage, accessModePage}
	if workload == catalog.WorkloadHTTPAPI {
		pages = []page{topologyPage, architecturePage, capabilityPage, generationModePage, providerPage, accessModePage}
	}
	selected -= 4
	if selected < 0 || selected >= len(pages) {
		return editOptionsPage
	}
	return pages[selected]
}

type blueprintEditRoute struct {
	page           page
	selectedRow    int
	returnToReview bool
}

func blueprintEditRouteAt(selected int, workload string, selectedWorkload, shape, architecture, mode, access int) blueprintEditRoute {
	target := blueprintEditPageAt(selected, workload)
	route := blueprintEditRoute{page: target, returnToReview: target != editOptionsPage}
	switch target {
	case workloadPage:
		route.selectedRow = selectedWorkload
	case topologyPage:
		route.selectedRow = topologyIndex(shape)
	case architecturePage:
		route.selectedRow = architecture
	case generationModePage:
		route.selectedRow = mode
	case accessModePage:
		route.selectedRow = access
	}
	return route
}

func prepareBlueprintEdit(route blueprintEditRoute, options *generationOptions, mode, provider *int, accessReturn *page, editingLocation *bool) {
	switch route.page {
	case projectLocationPage:
		*editingLocation = true
	case generationModePage:
		*mode = route.selectedRow
	case providerPage:
		options.Mode = "agent"
		*mode = generationModeIndex(*options)
		*provider = providerIndex(options.Provider)
	case accessModePage:
		*accessReturn = summaryPage
	}
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
