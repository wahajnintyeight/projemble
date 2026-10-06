package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

type page int

const (
	projectNamePage page = iota
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
)

func Run() error {
	return runWithInitializer(ui.Init)
}

func runWithInitializer(initialize func() error) error {
	if err := initialize(); err != nil {
		return fmt.Errorf("initialize terminal UI: %w", err)
	}
	defer ui.Close()

	currentPage := projectNamePage
	lastPage := page(-1)
	selectedShape := 0
	selectedArchitecture := 0
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
		case projectNamePage:
			setInputLayout(nameInput, width, height)
			if validationMessage != "" {
				nameInput.TitleBottom = validationMessage
			} else {
				nameInput.TitleBottom = "Enter continue     Esc quit"
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
			updateSummary(list, nameInput.Text, descriptionInput.Text, template, projectPath, "Enter save to YAML", saveError, width, height)
			ui.Render(list)
		case savedPage:
			template, ok := templateForChoices(selectedShape, selectedArchitecture)
			if !ok {
				return fmt.Errorf("selected shape and architecture have no matching template")
			}
			updateSummary(list, nameInput.Text, descriptionInput.Text, template, projectPath, "", "", width, height)
			list.Title = "Project profile saved"
			list.Rows = append(list.Rows,
				"",
				"Config: "+configPath,
				"No source files have been generated yet.",
				"Press q to exit.",
			)
			list.TitleBottom = "q  Exit"
			ui.Render(list)
		}

		event := <-uiEvents
		if currentPage == projectNamePage || currentPage == projectDescriptionPage || currentPage == projectLocationPage {
			var nextPage page
			advance, quit, message := handleTextInput(event, currentPage, nameInput, descriptionInput, locationInput, &nextPage)
			validationMessage = message
			if quit {
				return nil
			}
			if advance {
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
		case "<Up>", "k":
			if currentPage == appShapePage || currentPage == architecturePage {
				list.ScrollUp()
				if currentPage == appShapePage {
					selectedShape = list.SelectedRow
				} else {
					selectedArchitecture = list.SelectedRow
				}
			}
		case "<Down>", "j":
			if currentPage == appShapePage || currentPage == architecturePage {
				list.ScrollDown()
				if currentPage == appShapePage {
					selectedShape = list.SelectedRow
				} else {
					selectedArchitecture = list.SelectedRow
				}
			}
		case "<Escape>", "b", "<Backspace>":
			switch currentPage {
			case appShapePage:
				currentPage = projectDescriptionPage
			case architecturePage:
				currentPage = appShapePage
			case summaryPage:
				currentPage = architecturePage
			}
		case "<Enter>":
			switch currentPage {
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
				savedPath, savedConfig, err := saveProject(nameInput.Text, descriptionInput.Text, projectPath, template)
				if err != nil {
					saveError = "Save failed: " + err.Error()
					continue
				}
				projectPath = savedPath
				configPath = savedConfig
				saveError = ""
				currentPage = savedPage
			}
		}
	}
}

func handleTextInput(event ui.Event, currentPage page, nameInput, descriptionInput, locationInput *widgets.Input, nextPage *page) (advance, quit bool, message string) {
	active := nameInput
	if currentPage == projectDescriptionPage {
		active = descriptionInput
	} else if currentPage == projectLocationPage {
		active = locationInput
	}
	switch event.ID {
	case "<C-c>":
		return false, true, ""
	case "<Escape>":
		if currentPage == projectNamePage {
			return false, true, ""
		}
		if currentPage == projectLocationPage {
			*nextPage = projectDescriptionPage
		} else {
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

func updateSummary(list *widgets.List, name, description string, template catalog.Template, projectPath, action, saveError string, width, height int) {
	shape, _ := catalog.AppShapeByID(template.AppShapeID)
	architecture, _ := catalog.ArchitectureByID(template.ArchitectureID)
	list.Title = "Review project profile"
	list.Rows = []string{
		"Project name:  " + name,
		"Description:   " + description,
		"Template:      " + template.Name,
		"App shape:     " + shape.Name,
		"Architecture:  " + architecture.Name,
		"Stack:         " + template.StackID,
		"Service setup: " + frameworkLabel(template.ServiceFrameworkID),
		"Project path:  " + projectPath,
		"",
		"This saves the project profile in the local YAML config.",
		"Project source files will be generated in a later step.",
		"",
	}
	if saveError != "" {
		list.Rows = append(list.Rows, saveError)
	}
	list.SelectedRow = 0
	list.SelectedStyle = list.TextStyle
	if action != "" {
		list.TitleBottom = action + " | n name d desc p path s shape a arch | q quit"
	} else {
		list.TitleBottom = "n name d desc p path s shape a arch | q quit"
	}
	list.SetRect(0, 0, width, height)
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
	if path == "" {
		return "", "", fmt.Errorf("project path is required")
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
	if err := projectstore.SaveDefault(config); err != nil {
		if cleanupErr := os.Remove(path); cleanupErr != nil {
			return "", "", fmt.Errorf("save project profile: %w (could not remove newly created empty project directory: %v)", err, cleanupErr)
		}
		return "", "", err
	}
	configPath, err := projectstore.DefaultPath()
	if err != nil {
		return "", "", err
	}
	return path, configPath, nil
}

func frameworkLabel(id string) string {
	if id == "" {
		return "none"
	}
	return id
}
