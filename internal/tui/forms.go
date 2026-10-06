package tui

import (
	"context"
	"errors"
	"fmt"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"os"
	"path/filepath"
	"projemble/internal/auth"
	"projemble/internal/catalog"
	"projemble/internal/llm"
	"projemble/internal/projectstore"
	"strings"
	"unicode/utf8"
)

func handleTextInput(event ui.Event, currentPage page, nameInput, descriptionInput, locationInput, keyInput, modelInput *widgets.Input, nextPage *page) (advance, quit bool, message string) {
	active := nameInput
	if currentPage == apiKeyPage {
		active = keyInput
	} else if currentPage == aiModelPage {
		active = modelInput
	} else if currentPage == projectDescriptionPage {
		active = descriptionInput
	} else if currentPage == projectLocationPage || currentPage == repairPathPage {
		active = locationInput
	}
	switch event.ID {
	case "<C-c>":
		return false, true, ""
	case "<Escape>", "<Esc>", "<Key:Escape>", "<Key:27>", "Escape", "<C-[>", "<C-b>", "\x1b":
		switch currentPage {
		case apiKeyPage:
			*nextPage = providerPage
		case aiModelPage:
			*nextPage = apiKeyPage
		case projectNamePage:
			*nextPage = homePage
		case projectLocationPage:
			*nextPage = projectDescriptionPage
		case repairPathPage:
			*nextPage = homePage
		default:
			*nextPage = projectNamePage
		}
		return true, false, ""
	case "<Enter>":
		if currentPage == projectNamePage {
			name := strings.TrimSpace(active.Text)
			if err := validateProjectName(name); err != nil {
				return false, false, err.Error()
			}
			active.Text = name
			active.Cursor = utf8.RuneCountInString(name)
			*nextPage = projectDescriptionPage
			return true, false, ""
		}
		if currentPage == projectDescriptionPage {
			description := strings.TrimSpace(active.Text)
			if description == "" {
				return false, false, "Add a short description to continue."
			}
			if utf8.RuneCountInString(description) > maxDescriptionRunes {
				return false, false, fmt.Sprintf("Keep the description under %d characters.", maxDescriptionRunes)
			}
			active.Text = description
			active.Cursor = utf8.RuneCountInString(description)
			*nextPage = projectLocationPage
			return true, false, ""
		}
		if currentPage == apiKeyPage {
			key := strings.TrimSpace(active.Text)
			if key == "" {
				return false, false, "Enter an API key to continue."
			}
			if strings.ContainsAny(key, "\r\n") {
				return false, false, "API keys cannot contain line breaks."
			}
			if utf8.RuneCountInString(key) > maxAPIKeyRunes {
				return false, false, "API key is too long."
			}
			active.Text = key
			active.Cursor = utf8.RuneCountInString(key)
			*nextPage = aiModelPage
			return true, false, ""
		}
		if currentPage == aiModelPage {
			model := strings.TrimSpace(active.Text)
			if model == "" {
				return false, false, "Enter a model ID to continue."
			}
			active.Text = model
			active.Cursor = utf8.RuneCountInString(model)
			*nextPage = summaryPage
			return true, false, ""
		}
		if currentPage == repairPathPage {
			enteredPath := strings.TrimSpace(active.Text)
			if !filepath.IsAbs(enteredPath) {
				return false, false, "Enter an absolute project directory path."
			}
			path, err := filepath.Abs(filepath.Clean(enteredPath))
			if err != nil {
				return false, false, "Enter a valid absolute project path."
			}
			info, err := os.Stat(path)
			if err != nil {
				return false, false, "Project directory does not exist: " + path
			}
			if !info.IsDir() {
				return false, false, "Project path must be an existing directory."
			}
			active.Text = path
			active.Cursor = utf8.RuneCountInString(path)
			*nextPage = homePage
			return true, false, ""
		}
		parent, err := validateProjectParent(active.Text)
		if err != nil {
			return false, false, err.Error()
		}
		active.Text = parent
		active.Cursor = utf8.RuneCountInString(parent)
		*nextPage = appShapePage
		return true, false, ""
	case "<Backspace>", "<C-h>":
		active.Backspace()
	case "<Left>":
		active.MoveCursorLeft()
	case "<Right>":
		active.MoveCursorRight()
	case "<Home>", "<C-a>":
		active.Cursor = 0
	case "<End>", "<C-e>":
		active.Cursor = utf8.RuneCountInString(active.Text)
	case "<Space>":
		insertRune(active, ' ', currentPage)
	default:
		if event.Type == ui.KeyboardEvent {
			runes := []rune(event.ID)
			if len(runes) == 1 {
				insertRune(active, runes[0], currentPage)
			}
		}
	}
	return false, false, ""
}

func isEscapeKey(id string) bool {
	return id == "<Escape>" || id == "<Esc>" || id == "<Key:Escape>" || id == "<Key:27>" || id == "Escape" || id == "<C-[>" || id == "<C-b>" || id == "\x1b"
}

func insertRune(input *widgets.Input, value rune, currentPage page) {
	limit := maxProjectNameRunes
	switch currentPage {
	case projectDescriptionPage:
		limit = maxDescriptionRunes
	case projectLocationPage, repairPathPage:
		limit = maxProjectPathRunes
	case apiKeyPage:
		limit = maxAPIKeyRunes
	case aiModelPage:
		limit = maxModelRunes
	}
	if utf8.RuneCountInString(input.Text) < limit {
		input.InsertRune(value)
	}
}

func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("Project name is required.")
	}
	if utf8.RuneCountInString(name) > maxProjectNameRunes {
		return fmt.Errorf("Keep the project name under %d characters.", maxProjectNameRunes)
	}
	if name == "." || name == ".." || strings.HasSuffix(name, ".") || strings.ContainsAny(name, `/\\<>:"|?*`) || isWindowsReservedName(name) {
		return fmt.Errorf("Use a project name without path or reserved filename characters.")
	}
	return nil
}

func isWindowsReservedName(name string) bool {
	base, _, _ := strings.Cut(strings.ToUpper(name), ".")
	switch base {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

func setInputLayout(input *widgets.Input, width, height int) {
	boxWidth := min(width, 76)
	left := max(0, (width-boxWidth)/2)
	top := max(0, height/2-2)
	input.SetRect(left, top, left+boxWidth, min(height, top+4))
}

func appShapeChoices() []catalogChoice {
	choices := catalog.AppShapes()
	result := make([]catalogChoice, 0, len(choices))
	for _, choice := range choices {
		result = append(result, catalogChoice{name: choice.Name, description: choice.Description})
	}
	return result
}

func appShapeIDAt(index int) string {
	choices := catalog.AppShapes()
	if index < 0 || index >= len(choices) {
		return ""
	}
	return choices[index].ID
}

func clampArchitectureIndex(shapeIndex, architectureIndex int) int {
	architectures := catalog.ArchitecturesForShape(appShapeIDAt(shapeIndex))
	if len(architectures) == 0 {
		return 0
	}
	return min(max(architectureIndex, 0), len(architectures)-1)
}

func architectureChoices(shapeID string) []catalogChoice {
	choices := catalog.ArchitecturesForShape(shapeID)
	result := make([]catalogChoice, 0, len(choices))
	for _, choice := range choices {
		result = append(result, catalogChoice{name: choice.Name, description: choice.Description})
	}
	return result
}

type catalogChoice struct {
	name        string
	description string
}

type providerChoice struct {
	id   llm.ProviderID
	name string
	env  string
}

type generationOptions struct {
	Mode        string
	Provider    llm.ProviderID
	Model       string
	APIKey      string
	Credentials llm.TokenSource
}

type chatGPTAuthResult struct {
	source *auth.ChatGPTTokenSource
	err    error
}

func generationModeChoices() []catalogChoice {
	return []catalogChoice{
		{name: "Scaffold only", description: "Generate the selected template offline. No account or API key needed."},
		{name: "Scaffold + agent", description: "Generate the same template, then let your agent implement features and run checks."},
	}
}

func providerChoices() []catalogChoice {
	choices := providerCatalog()
	result := make([]catalogChoice, 0, len(choices))
	for _, choice := range choices {
		description := "API key required"
		if choice.id == llm.OpenAIWeb {
			description = "Eligible Plus/Pro accounts; free accounts can use Local templates, an OpenAI API key, or another provider"
		}
		result = append(result, catalogChoice{name: choice.name, description: description})
	}
	return result
}

func providerCatalog() []providerChoice {
	return []providerChoice{
		{id: llm.OpenAI, name: "OpenAI", env: "OPENAI_API_KEY"},
		{id: llm.Claude, name: "Claude", env: "ANTHROPIC_API_KEY"},
		{id: llm.DeepSeek, name: "DeepSeek", env: "DEEPSEEK_API_KEY"},
		{id: llm.Mistral, name: "Mistral", env: "MISTRAL_API_KEY"},
		{id: llm.Qwen, name: "Qwen", env: "DASHSCOPE_API_KEY"},
		{id: llm.OpenRouter, name: "OpenRouter", env: "OPENROUTER_API_KEY"},
		{id: llm.HuggingFace, name: "Hugging Face", env: "HF_TOKEN"},
		{id: llm.Gemini, name: "Gemini", env: "GEMINI_API_KEY"},
		{id: llm.OpenAIWeb, name: "ChatGPT plan (eligible Plus/Pro)"},
	}
}

func providerAt(index int) (providerChoice, bool) {
	choices := providerCatalog()
	if index < 0 || index >= len(choices) {
		return providerChoice{}, false
	}
	return choices[index], true
}

func isChoicePage(current page) bool {
	return current == homePage || current == generationModePage || current == providerPage || current == appShapePage || current == architecturePage
}

func updateChoiceList(list *widgets.List, title string, choices []catalogChoice, selected, width, height int) {
	list.Title = title
	themeList(list)
	list.Rows = make([]string, 0, len(choices))
	for _, choice := range choices {
		list.Rows = append(list.Rows, styleLabel(choice.name)+" - "+choice.description)
	}
	list.SelectedRow = selected
	setFooter(&list.Block, "Up/Down or j/k  Move     Enter  Continue     b  Back     q  Quit", false)
	list.SetRect(0, 0, width, height)
}

func updateSummary(list *widgets.List, name, description string, template catalog.Template, projectPath string, options generationOptions, action, saveError string, width, height int) {
	shape, _ := catalog.AppShapeByID(template.AppShapeID)
	architecture, _ := catalog.ArchitectureByID(template.ArchitectureID)
	credentialNote := "Local generation needs no provider credentials."
	if options.Mode == "agent" && options.Provider == llm.OpenAIWeb {
		credentialNote = "ChatGPT sign-in is stored separately from project YAML."
	} else if options.Mode == "agent" {
		credentialNote = "Provider API key is saved in local YAML config."
	}
	list.Title = "Your project blueprint / Ready to build"
	list.Rows = []string{
		styleLabel("Project name:") + "  " + name,
		styleLabel("Description:") + "   " + description,
		styleLabel("Generation:") + "    " + generationLabel(options),
		styleLabel("Template:") + "      " + template.Name,
		styleLabel("App shape:") + "     " + shape.Name,
		styleLabel("Architecture:") + "  " + architecture.Name,
		styleLabel("Stack:") + "         " + template.StackID,
		styleLabel("Service setup:") + " " + frameworkLabel(template.ServiceFrameworkID),
		styleLabel("Project path:") + "  " + projectPath,
		"",
		"Projemble generates this template and saves your project blueprint locally.",
		credentialNote,
		"",
	}
	if saveError != "" {
		list.Rows = append(list.Rows, styleError(saveError))
	}
	list.SelectedRow = 0
	list.SelectedStyle = list.TextStyle
	if action != "" {
		setFooter(&list.Block, action+" | r provider g generation n name d desc p path s shape a arch | q quit", false)
	} else {
		setFooter(&list.Block, "r provider g generation n name d desc p path s shape a arch | q quit", false)
	}
	list.SetRect(0, 0, width, height)
}

func generationLabel(options generationOptions) string {
	if options.Mode != "agent" {
		return "Scaffold only"
	}
	choice, _ := providerAtIndex(options.Provider)
	return fmt.Sprintf("Scaffold + agent · %s · %s", choice, options.Model)
}

func initialGenerationOptions(config projectstore.Config) generationOptions {
	defaults := config.Generation
	if defaults.Mode == "" && len(config.Projects) > 0 {
		latest := config.Projects[0]
		for _, project := range config.Projects[1:] {
			if project.UpdatedAt >= latest.UpdatedAt {
				latest = project
			}
		}
		defaults = projectstore.GenerationDefaults{Mode: latest.GenerationMode, Provider: latest.AIProvider, Model: latest.AIModel}
	}
	if defaults.Mode != "agent" {
		return generationOptions{Mode: "local"}
	}
	return generationOptions{Mode: "agent", Provider: llm.ProviderID(defaults.Provider), Model: defaults.Model}
}

func providerIndex(id llm.ProviderID) int {
	for index, choice := range providerCatalog() {
		if choice.id == id {
			return index
		}
	}
	return 0
}

func chatGPTSignInMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "ChatGPT sign-in timed out"
	}
	return "ChatGPT sign-in failed: " + err.Error()
}

func providerAtIndex(id llm.ProviderID) (string, bool) {
	for _, choice := range providerCatalog() {
		if choice.id == id {
			return choice.name, true
		}
	}
	return string(id), false
}

func templateForChoices(shapeIndex, architectureIndex int) (catalog.Template, bool) {
	shapes := catalog.AppShapes()
	if shapeIndex < 0 || shapeIndex >= len(shapes) {
		return catalog.Template{}, false
	}
	architectures := catalog.ArchitecturesForShape(shapes[shapeIndex].ID)
	if architectureIndex < 0 || architectureIndex >= len(architectures) {
		return catalog.Template{}, false
	}
	for _, template := range catalog.Templates() {
		if template.AppShapeID == shapes[shapeIndex].ID && template.ArchitectureID == architectures[architectureIndex].ID {
			return template, true
		}
	}
	return catalog.Template{}, false
}

func frameworkLabel(id string) string {
	if id == "" {
		return "none"
	}
	return id
}
