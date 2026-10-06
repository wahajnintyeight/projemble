package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"

	"projemble/internal/agent"
	"projemble/internal/auth"
	"projemble/internal/catalog"
	"projemble/internal/generator"
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
	appShapePage
	architecturePage
	summaryPage
	savedPage
)

const (
	maxProjectNameRunes = 60
	maxDescriptionRunes = 160
	maxProjectPathRunes = 4096
	maxAPIKeyRunes      = 4096
	maxModelRunes       = 128
)

func Run() error {
	return runWithInitializer(ui.Init)
}

func runWithInitializer(initialize func() error) error {
	config, err := projectstore.LoadDefault()
	if err != nil {
		return fmt.Errorf("load saved Projemble settings: %w", err)
	}
	if err := initialize(); err != nil {
		return fmt.Errorf("initialize terminal UI: %w", err)
	}
	defer ui.Close()

	currentPage := generationModePage
	lastPage := page(-1)
	selectedShape := 0
	selectedArchitecture := 0
	providerConfig := initialGenerationOptions(config)
	selectedMode := 0
	if providerConfig.Mode == "agent" {
		selectedMode = 1
	}
	selectedProvider := providerIndex(providerConfig.Provider)
	editingLocationFromSummary := false
	validationMessage := ""
	saveError := ""
	projectPath := ""
	configPath := ""
	nameInput := widgets.NewInput()
	nameInput.Border = true
	nameInput.Title = "Project name"
	nameInput.Placeholder = "Enter a unique project name"
	descriptionInput := widgets.NewInput()
	descriptionInput.Border = true
	descriptionInput.Title = "Short description"
	descriptionInput.Placeholder = "What will this project do?"
	locationInput := widgets.NewInput()
	locationInput.Border = true
	locationInput.Title = "Project parent directory"
	locationInput.Placeholder = "Absolute path; the project name is appended"
	keyInput := widgets.NewInput()
	keyInput.Border = true
	keyInput.Title = "Provider API key"
	keyInput.Placeholder = "Paste your key; it stays masked and is not saved"
	keyInput.EchoMode = widgets.EchoPassword
	modelInput := widgets.NewInput()
	modelInput.Border = true
	modelInput.Title = "Model ID"
	modelInput.Placeholder = "Model name from your provider, e.g. gpt-..."
	modelInput.Text = providerConfig.Model
	modelInput.Cursor = utf8.RuneCountInString(modelInput.Text)
	list := widgets.NewList()
	list.Border = true
	list.WrapText = true
	list.SelectedStyle = ui.NewStyle(ui.ColorWhite, ui.ColorBlue)
	uiEvents := ui.PollEvents()

	for {
		width, height := ui.TerminalDimensions()
		if currentPage != lastPage {
			ui.Clear()
			lastPage = currentPage
		}

		switch currentPage {
		case generationModePage:
			updateChoiceList(list, "How should Projemble generate your starter?", generationModeChoices(), selectedMode, width, height)
			list.TitleBottom = "Loaded default: " + generationLabel(providerConfig) + " | Up/Down or j/k  Select  Enter  Continue  q  Quit"
			ui.Render(list)
		case providerPage:
			updateChoiceList(list, "Choose an AI provider", providerChoices(), selectedProvider, width, height)
			if validationMessage != "" {
				list.TitleBottom = validationMessage
			}
			ui.Render(list)
		case apiKeyPage:
			setInputLayout(keyInput, width, height)
			if validationMessage != "" {
				keyInput.TitleBottom = validationMessage
			} else {
				choice, _ := providerAt(selectedProvider)
				keyInput.TitleBottom = "Enter continue | masked | " + choice.env + " is used if set | Esc back"
			}
			ui.Render(keyInput)
		case aiModelPage:
			setInputLayout(modelInput, width, height)
			if validationMessage != "" {
				modelInput.TitleBottom = validationMessage
			} else {
				modelInput.TitleBottom = "Enter continue | enter a model ID supported by your provider | Esc back"
			}
			ui.Render(modelInput)
		case projectNamePage:
			setInputLayout(nameInput, width, height)
			if validationMessage != "" {
				nameInput.TitleBottom = validationMessage
			} else {
				nameInput.TitleBottom = "Enter continue     Esc back"
			}
			ui.Render(nameInput)
		case projectDescriptionPage:
			setInputLayout(descriptionInput, width, height)
			if validationMessage != "" {
				descriptionInput.TitleBottom = validationMessage
			} else {
				descriptionInput.TitleBottom = "Enter continue     Esc back"
			}
			ui.Render(descriptionInput)
		case projectLocationPage:
			setInputLayout(locationInput, width, height)
			if validationMessage != "" {
				locationInput.TitleBottom = validationMessage
			} else {
				locationInput.TitleBottom = "Enter continue | project name is appended | Esc back"
			}
			ui.Render(locationInput)
		case appShapePage:
			updateChoiceList(list, "Choose the application shape", appShapeChoices(), selectedShape, width, height)
			ui.Render(list)
		case architecturePage:
			updateChoiceList(list, "Choose the architecture", architectureChoices(), selectedArchitecture, width, height)
			ui.Render(list)
		case summaryPage:
			template, ok := templateForChoices(selectedShape, selectedArchitecture)
			if !ok {
				return fmt.Errorf("selected shape and architecture have no matching template")
			}
			updateSummary(list, nameInput.Text, descriptionInput.Text, template, projectPath, providerConfig, "Enter generate project", saveError, width, height)
			ui.Render(list)
		case savedPage:
			template, ok := templateForChoices(selectedShape, selectedArchitecture)
			if !ok {
				return fmt.Errorf("selected shape and architecture have no matching template")
			}
			updateSummary(list, nameInput.Text, descriptionInput.Text, template, projectPath, providerConfig, "", "", width, height)
			list.Title = "Project profile saved"
			list.Rows = append(list.Rows,
				"",
				"Config: "+configPath,
				"Starter source files were generated.",
				"Press q to exit.",
			)
			list.TitleBottom = "q  Exit"
			ui.Render(list)
		}

		event := <-uiEvents
		if currentPage == apiKeyPage || currentPage == aiModelPage || currentPage == projectNamePage || currentPage == projectDescriptionPage || currentPage == projectLocationPage {
			var nextPage page
			advance, quit, message := handleTextInput(event, currentPage, nameInput, descriptionInput, locationInput, keyInput, modelInput, &nextPage)
			if currentPage == aiModelPage && event.ID == "<Escape>" && providerConfig.Provider == llm.OpenAIWeb {
				nextPage = providerPage
			}
			validationMessage = message
			if quit {
				return nil
			}
			if advance {
				switch currentPage {
				case apiKeyPage:
					providerConfig.APIKey = keyInput.Text
				case aiModelPage:
					providerConfig.Model = modelInput.Text
				}
				if currentPage == projectLocationPage && editingLocationFromSummary && event.ID == "<Enter>" {
					plannedPath, err := plannedProjectPath(locationInput.Text, nameInput.Text)
					if err != nil {
						validationMessage = err.Error()
						continue
					}
					projectPath = plannedPath
					editingLocationFromSummary = false
					currentPage = summaryPage
					validationMessage = ""
					continue
				}
				if currentPage == projectLocationPage && event.ID == "<Escape>" {
					editingLocationFromSummary = false
				}
				currentPage = nextPage
				validationMessage = ""
			}
			continue
		}

		switch event.ID {
		case "q", "<C-c>":
			return nil
		case "n":
			if currentPage == summaryPage {
				currentPage = projectNamePage
			}
		case "d":
			if currentPage == summaryPage {
				currentPage = projectDescriptionPage
			}
		case "p":
			if currentPage == summaryPage {
				editingLocationFromSummary = true
				currentPage = projectLocationPage
			}
		case "s":
			if currentPage == summaryPage {
				currentPage = appShapePage
			}
		case "a":
			if currentPage == summaryPage {
				currentPage = architecturePage
			}
		case "g":
			if currentPage == summaryPage {
				currentPage = generationModePage
			}
		case "r":
			if currentPage == summaryPage {
				providerConfig.Mode = "agent"
				currentPage = providerPage
			}
		case "<Up>", "k":
			if isChoicePage(currentPage) {
				list.ScrollUp()
				switch currentPage {
				case generationModePage:
					selectedMode = list.SelectedRow
				case providerPage:
					selectedProvider = list.SelectedRow
				case appShapePage:
					selectedShape = list.SelectedRow
				case architecturePage:
					selectedArchitecture = list.SelectedRow
				}
			}
		case "<Down>", "j":
			if isChoicePage(currentPage) {
				list.ScrollDown()
				switch currentPage {
				case generationModePage:
					selectedMode = list.SelectedRow
				case providerPage:
					selectedProvider = list.SelectedRow
				case appShapePage:
					selectedShape = list.SelectedRow
				case architecturePage:
					selectedArchitecture = list.SelectedRow
				}
			}
		case "<Escape>", "b", "<Backspace>":
			switch currentPage {
			case appShapePage:
				currentPage = projectLocationPage
			case providerPage:
				currentPage = generationModePage
			case generationModePage:
				return nil
			case architecturePage:
				currentPage = appShapePage
			case summaryPage:
				currentPage = architecturePage
			}
		case "<Enter>":
			switch currentPage {
			case generationModePage:
				selectedMode = list.SelectedRow
				if selectedMode == 0 {
					providerConfig.Mode = "local"
					currentPage = projectNamePage
				} else {
					providerConfig.Mode = "agent"
					currentPage = providerPage
				}
			case providerPage:
				selectedProvider = list.SelectedRow
				choice, ok := providerAt(selectedProvider)
				if !ok {
					validationMessage = "Choose a provider to continue."
					continue
				}
				providerChanged := choice.id != providerConfig.Provider
				providerConfig.Provider = choice.id
				providerConfig.APIKey = ""
				if providerChanged {
					providerConfig.Model = ""
					modelInput.Text = ""
					modelInput.Cursor = 0
				}
				if choice.id == llm.OpenAIWeb {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
					tokenSource, err := auth.EnsureChatGPT(ctx, io.Discard)
					cancel()
					if err != nil {
						validationMessage = "ChatGPT sign-in failed: " + err.Error()
						continue
					}
					providerConfig.Credentials = tokenSource
					modelInput.Placeholder = "Model ID available to your ChatGPT plan"
					currentPage = aiModelPage
				} else {
					keyInput.Text = os.Getenv(choice.env)
					keyInput.Cursor = utf8.RuneCountInString(keyInput.Text)
					keyInput.TitleBottom = ""
					currentPage = apiKeyPage
				}
				validationMessage = ""
			case appShapePage:
				selectedShape = list.SelectedRow
				currentPage = architecturePage
			case architecturePage:
				selectedArchitecture = list.SelectedRow
				plannedPath, err := plannedProjectPath(locationInput.Text, nameInput.Text)
				if err != nil {
					validationMessage = err.Error()
					currentPage = projectLocationPage
					continue
				}
				projectPath = plannedPath
				currentPage = summaryPage
			case summaryPage:
				template, ok := templateForChoices(selectedShape, selectedArchitecture)
				if !ok {
					saveError = "Select a valid shape and architecture before saving."
					continue
				}
				if providerConfig.Mode == "agent" {
					list.Title = "AI agent is generating your starter"
					list.Rows = []string{"Provider: " + generationLabel(providerConfig), "Project: " + nameInput.Text, "", "The agent is working in the new project folder.", "This can take a few minutes."}
					list.TitleBottom = "Please wait"
					list.SetRect(0, 0, width, height)
					ui.Render(list)
				}
				savedPath, savedConfig, err := saveProjectWithGeneration(nameInput.Text, descriptionInput.Text, projectPath, template, providerConfig)
				if err != nil {
					errorText := err.Error()
					if providerConfig.APIKey != "" {
						errorText = strings.ReplaceAll(errorText, providerConfig.APIKey, "[REDACTED]")
					}
					saveError = "Generation failed: " + errorText
					continue
				}
				projectPath = savedPath
				configPath = savedConfig
				providerConfig.APIKey = ""
				keyInput.Text = ""
				saveError = ""
				currentPage = savedPage
			}
		}
	}
}

func handleTextInput(event ui.Event, currentPage page, nameInput, descriptionInput, locationInput, keyInput, modelInput *widgets.Input, nextPage *page) (advance, quit bool, message string) {
	active := nameInput
	if currentPage == apiKeyPage {
		active = keyInput
	} else if currentPage == aiModelPage {
		active = modelInput
	} else if currentPage == projectDescriptionPage {
		active = descriptionInput
	} else if currentPage == projectLocationPage {
		active = locationInput
	}
	switch event.ID {
	case "<C-c>":
		return false, true, ""
	case "<Escape>":
		switch currentPage {
		case apiKeyPage:
			*nextPage = providerPage
		case aiModelPage:
			*nextPage = apiKeyPage
		case projectNamePage:
			*nextPage = generationModePage
		case projectLocationPage:
			*nextPage = projectDescriptionPage
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
			*nextPage = projectNamePage
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

func insertRune(input *widgets.Input, value rune, currentPage page) {
	limit := maxProjectNameRunes
	switch currentPage {
	case projectDescriptionPage:
		limit = maxDescriptionRunes
	case projectLocationPage:
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

func architectureChoices() []catalogChoice {
	choices := catalog.Architectures()
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

func generationModeChoices() []catalogChoice {
	return []catalogChoice{
		{name: "Local templates", description: "Generate the selected starter directly. No account, API key, or network needed."},
		{name: "AI-assisted generation", description: "Have your chosen AI provider build on the selected starter."},
	}
}

func providerChoices() []catalogChoice {
	choices := providerCatalog()
	result := make([]catalogChoice, 0, len(choices))
	for _, choice := range choices {
		description := "API key required"
		if choice.id == llm.OpenAIWeb {
			description = "Sign in with ChatGPT in your browser; no API key"
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
		{id: llm.OpenAIWeb, name: "ChatGPT plan"},
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
	return current == generationModePage || current == providerPage || current == appShapePage || current == architecturePage
}

func updateChoiceList(list *widgets.List, title string, choices []catalogChoice, selected, width, height int) {
	list.Title = title
	list.SelectedStyle = ui.NewStyle(ui.ColorWhite, ui.ColorBlue)
	list.Rows = make([]string, 0, len(choices))
	for _, choice := range choices {
		list.Rows = append(list.Rows, choice.name+" - "+choice.description)
	}
	list.SelectedRow = selected
	list.TitleBottom = "Up/Down or j/k  Move     Enter  Continue     b  Back     q  Quit"
	list.SetRect(0, 0, width, height)
}

func updateSummary(list *widgets.List, name, description string, template catalog.Template, projectPath string, options generationOptions, action, saveError string, width, height int) {
	shape, _ := catalog.AppShapeByID(template.AppShapeID)
	architecture, _ := catalog.ArchitectureByID(template.ArchitectureID)
	credentialNote := "Local generation needs no provider credentials."
	if options.Mode == "agent" && options.Provider == llm.OpenAIWeb {
		credentialNote = "ChatGPT sign-in is stored separately from project YAML."
	} else if options.Mode == "agent" {
		credentialNote = "Provider API key is used for this run and never saved in YAML."
	}
	list.Title = "Review project profile"
	list.Rows = []string{
		"Project name:  " + name,
		"Description:   " + description,
		"Generation:    " + generationLabel(options),
		"Template:      " + template.Name,
		"App shape:     " + shape.Name,
		"Architecture:  " + architecture.Name,
		"Stack:         " + template.StackID,
		"Service setup: " + frameworkLabel(template.ServiceFrameworkID),
		"Project path:  " + projectPath,
		"",
		"This creates the starter project and saves its profile in local YAML.",
		credentialNote,
		"",
	}
	if saveError != "" {
		list.Rows = append(list.Rows, saveError)
	}
	list.SelectedRow = 0
	list.SelectedStyle = list.TextStyle
	if action != "" {
		list.TitleBottom = action + " | r provider g generation n name d desc p path s shape a arch | q quit"
	} else {
		list.TitleBottom = "r provider g generation n name d desc p path s shape a arch | q quit"
	}
	list.SetRect(0, 0, width, height)
}

func generationLabel(options generationOptions) string {
	if options.Mode != "agent" {
		return "Local templates"
	}
	choice, _ := providerAtIndex(options.Provider)
	return fmt.Sprintf("AI-assisted · %s · %s", choice, options.Model)
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
	architectures := catalog.Architectures()
	if shapeIndex < 0 || shapeIndex >= len(shapes) || architectureIndex < 0 || architectureIndex >= len(architectures) {
		return catalog.Template{}, false
	}
	for _, template := range catalog.Templates() {
		if template.AppShapeID == shapes[shapeIndex].ID && template.ArchitectureID == architectures[architectureIndex].ID {
			return template, true
		}
	}
	return catalog.Template{}, false
}

func saveProject(name, description, path string, template catalog.Template) (string, string, error) {
	return saveProjectWithGeneration(name, description, path, template, generationOptions{Mode: "local"})
}

func saveProjectWithGeneration(name, description, path string, template catalog.Template, options generationOptions) (string, string, error) {
	if path == "" {
		return "", "", fmt.Errorf("project path is required")
	}
	if options.Mode != "agent" {
		options = generationOptions{Mode: "local"}
	}
	if options.Mode == "agent" {
		if strings.TrimSpace(options.Model) == "" {
			return "", "", fmt.Errorf("AI model ID is required")
		}
		if options.Provider != llm.OpenAIWeb && strings.TrimSpace(options.APIKey) == "" {
			return "", "", fmt.Errorf("provider API key is required")
		}
		if options.Provider == llm.OpenAIWeb && options.Credentials == nil {
			return "", "", fmt.Errorf("ChatGPT sign-in is required")
		}
	}
	if filepath.Base(filepath.Clean(path)) != name {
		return "", "", fmt.Errorf("project path must end with the project name %q", name)
	}
	if _, err := validateProjectParent(filepath.Dir(path)); err != nil {
		return "", "", err
	}
	config, err := projectstore.LoadDefault()
	if err != nil {
		return "", "", err
	}
	config.Generation = projectstore.GenerationDefaults{Mode: options.Mode, Provider: string(options.Provider), Model: options.Model}
	settings := make(map[string]string)
	if template.ServiceFrameworkID != "" {
		settings[catalog.SettingServiceFramework] = template.ServiceFrameworkID
	}
	project := projectstore.Project{
		Name:           name,
		Description:    description,
		Path:           path,
		StackID:        template.StackID,
		AppShapeID:     template.AppShapeID,
		ArchitectureID: template.ArchitectureID,
		TemplateID:     template.ID,
		GenerationMode: options.Mode,
		AIProvider:     string(options.Provider),
		AIModel:        options.Model,
		Settings:       settings,
	}
	if err := config.Upsert(project); err != nil {
		return "", "", err
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		if os.IsExist(err) {
			return "", "", fmt.Errorf("project directory already exists: %s", path)
		}
		return "", "", fmt.Errorf("create project directory %q: %w", path, err)
	}
	if err := generator.Generate(project); err != nil {
		if cleanupErr := os.RemoveAll(path); cleanupErr != nil {
			return "", "", fmt.Errorf("generate project files: %w (could not clean newly created project directory: %v)", err, cleanupErr)
		}
		return "", "", fmt.Errorf("generate project files: %w", err)
	}
	if options.Mode == "agent" {
		providerAgent, err := agent.New(agent.Config{ProviderID: options.Provider, Model: options.Model, APIKey: options.APIKey, Credentials: options.Credentials})
		if err != nil {
			_ = os.RemoveAll(path)
			return "", "", fmt.Errorf("configure AI agent: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
		err = providerAgent.Run(ctx, path, generationTask(project, template), io.Discard)
		cancel()
		if err != nil {
			if cleanupErr := os.RemoveAll(path); cleanupErr != nil {
				return "", "", fmt.Errorf("AI generation failed: %w (could not clean new project directory: %v)", err, cleanupErr)
			}
			return "", "", fmt.Errorf("AI generation failed: %w", err)
		}
	}
	if err := projectstore.SaveDefault(config); err != nil {
		if cleanupErr := os.RemoveAll(path); cleanupErr != nil {
			return "", "", fmt.Errorf("save project profile: %w (could not remove newly created project directory: %v)", err, cleanupErr)
		}
		return "", "", err
	}
	configPath, err := projectstore.DefaultPath()
	if err != nil {
		return "", "", err
	}
	return path, configPath, nil
}

func generationTask(project projectstore.Project, template catalog.Template) string {
	return fmt.Sprintf(`Implement a useful first version of this project inside the existing starter workspace.

Project: %s
Purpose: %s
Stack: %s
Application shape: %s
Architecture: %s
Template: %s

Keep the selected architecture and starter structure. Add only the smallest coherent domain functionality that serves the stated purpose. Keep the code buildable, add focused tests, and run the available checks before finishing. Do not add external services, credentials, or dependencies unless the described purpose requires them.`, project.Name, project.Description, template.StackID, template.AppShapeID, template.ArchitectureID, template.Name)
}

func frameworkLabel(id string) string {
	if id == "" {
		return "none"
	}
	return id
}
