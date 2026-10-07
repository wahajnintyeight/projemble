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
	"projemble/internal/catalog"
	"projemble/internal/llm"
	"projemble/internal/projectstore"
)

func Run() error {
	return runWithInitializer(ui.Init)
}

func runWithInitializer(initialize func() error) error {
	config, err := projectstore.LoadDefault()
	if err != nil {
		return fmt.Errorf("load saved Projemble settings: %w", err)
	}
	restoreTheme := activateTUITheme()
	defer restoreTheme()
	if err := initialize(); err != nil {
		return fmt.Errorf("initialize terminal UI: %w", err)
	}
	defer ui.Close()

	currentPage := homePage
	selectedHome := 0
	for i, project := range config.Projects {
		if project.ID == config.LastProjectID {
			selectedHome = i + 1
		}
	}
	var reopening *projectstore.Project
	editingSettings := false
	lastPage := page(-1)
	selectedShape := 0
	selectedArchitecture := 0
	selectedWorkload := 0
	selectedStack := 0
	wizard := projectWizard{}
	selectedAccessMode := 2
	accessReturnPage := homePage
	var pendingPermission *generationUpdate
	providerConfig := initialGenerationOptions(config)
	selectedAccessMode = accessModeIndex(providerConfig.AccessMode)
	selectedMode := generationModeIndex(providerConfig)
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
	locationInput.Text = rememberedDirectory(config)
	locationInput.Cursor = utf8.RuneCountInString(locationInput.Text)
	locationInput.Placeholder = "Absolute path; the project name is appended"
	keyInput := widgets.NewInput()
	keyInput.Border = true
	keyInput.Title = "Provider API key"
	keyInput.Placeholder = "Paste your key; saved in local YAML config"
	keyInput.EchoMode = widgets.EchoPassword
	modelInput := widgets.NewInput()
	modelInput.Border = true
	modelInput.Title = "Model ID"
	modelInput.Placeholder = "Model name from your provider, e.g. gpt-..."
	modelInput.Text = providerConfig.Model
	modelInput.Cursor = utf8.RuneCountInString(modelInput.Text)
	for _, input := range []*widgets.Input{nameInput, descriptionInput, locationInput, keyInput, modelInput} {
		themeInput(input)
	}
	list := widgets.NewList()
	list.Border = true
	list.WrapText = true
	themeList(list)
	workspace := newAgentWorkspace()
	models := newModelPicker()
	defer models.close()
	var deferredNavigation *workspaceAction
	var previousOptions *generationOptions
	animationTicker := time.NewTicker(120 * time.Millisecond)
	defer animationTicker.Stop()
	uiEvents := ui.PollEvents()
	var authPending bool
	var authCancel context.CancelFunc
	var authResults chan chatGPTAuthResult
	var generationUpdates chan generationUpdate
	var generationCancel context.CancelFunc
	var generationRows []string
	var agentSession *agent.Agent
	var pendingPrompts []string
	agentSecret := ""
	generationPurpose := "project"
	generationRunning := false
	quitAfterGeneration := false
	followAgentActivity := true
	defer func() {
		if authCancel != nil {
			authCancel()
		}
		if generationCancel != nil {
			generationCancel()
		}
	}()
	startAgentTurn := func(prompt string) {
		generationPurpose = "turn"
		generationUpdates, generationCancel = launchAgentTurn(agentSession, projectPath, agentSecret, prompt)
		generationRunning = true
	}
	navigateWorkspace := func(action workspaceAction) {
		loaded, loadErr := projectstore.LoadDefault()
		if loadErr != nil {
			generationRows = appendActivity(generationRows, "Navigation error: "+loadErr.Error())
			return
		}
		config = loaded
		target := workspaceDestination(action)
		if target != homePage {
			models.custom = false
			if providerConfig.APIKey == "" {
				providerConfig.APIKey = rememberedKey(providerConfig.Provider, config)
			}
			previous := providerConfig
			previousOptions = &previous
			project, projectErr := projectForWorkspace(config, projectPath)
			if projectErr != nil {
				generationRows = appendActivity(generationRows, "Navigation error: "+projectErr.Error())
				return
			}
			reopening = project
			selectedProvider = providerIndex(providerConfig.Provider)
			modelInput.Text = providerConfig.Model
			modelInput.Cursor = utf8.RuneCountInString(modelInput.Text)
		}
		validationMessage = ""
		currentPage = target
	}

	for {
		width, height := ui.TerminalDimensions()
		if currentPage != lastPage {
			ui.Clear()
			lastPage = currentPage
		}

		view := wizardView{
			currentPage: currentPage, list: list, models: models,
			nameInput: nameInput, descriptionInput: descriptionInput, locationInput: locationInput, keyInput: keyInput, modelInput: modelInput,
			selectedMode: selectedMode, selectedProvider: selectedProvider, selectedShape: selectedShape, selectedArchitecture: selectedArchitecture, selectedWorkload: selectedWorkload, selectedStack: selectedStack,
			providerConfig: providerConfig, validationMessage: validationMessage, saveError: saveError, projectPath: projectPath, configPath: configPath,
			name: nameInput.Text, description: descriptionInput.Text, authPending: authPending, reopening: reopening, selection: &wizard,
		}
		handledView, viewErr := renderWizardPage(view, width, height)
		if viewErr != nil {
			return viewErr
		}
		if !handledView {
			switch currentPage {
			case homePage:
				renderHomePage(list, config, selectedHome, validationMessage, width, height)
			case agentProgressPage:
				workspace.Render(width, height, generationRows, providerConfig, projectPath, generationRunning, followAgentActivity, len(pendingPrompts))
			case accessModePage:
				updateChoiceList(list, "Agent access mode", accessModeChoices(), selectedAccessMode, width, height)
				setFooter(&list.Block, "Enter save to YAML | Esc back", false)
				renderOnboardingChoices(list, "Choose how Projemble may work inside your project. Ask always approves every action. Shell starts in this directory but the OS does not confine it, so each arbitrary shell command requires approval in every mode.", width, height)
			case approvalPage:
				if pendingPermission != nil && pendingPermission.permission != nil {
					renderPermissionPrompt(list, *pendingPermission.permission, list.SelectedRow, width, height)
				}
			}
		}

		var event ui.Event
		var animationEvents <-chan time.Time
		var mentionIndexEvents <-chan mentionIndexUpdate
		if currentPage == agentProgressPage && generationRunning {
			animationEvents = animationTicker.C
		}
		if currentPage == agentProgressPage {
			mentionIndexEvents = workspace.mentions.updates
		}
		select {
		case result := <-models.results:
			models.receive(result)
			continue
		case update := <-mentionIndexEvents:
			workspace.mentions.receive(update)
			continue
		case <-animationEvents:
			workspace.Tick()
			continue
		case event = <-uiEvents:
		case update := <-generationUpdates:
			if update.permission != nil {
				pendingPermission = &update
				accessReturnPage = agentProgressPage
				list.SelectedRow = 1
				currentPage = approvalPage
				continue
			}
			if update.hasUsage {
				workspace.AddUsage(update.usage)
				continue
			}
			if update.activity != "" {
				generationRows = appendActivity(generationRows, update.activity)
				continue
			}
			if update.done {
				generationUpdates = nil
				generationRunning = false
				if generationCancel != nil {
					generationCancel()
					generationCancel = nil
				}
				if generationPurpose == "project" && update.session != nil {
					agentSession = update.session
				}
				if update.err != nil {
					errorText := update.err.Error()
					secret := providerConfig.APIKey
					if agentSecret != "" {
						secret = agentSecret
					}
					if secret != "" {
						errorText = strings.ReplaceAll(errorText, secret, "[REDACTED]")
					}
					if generationPurpose == "project" {
						saveError = "Generation failed: " + errorText
						generationRows = appendActivity(generationRows, saveError)
					} else {
						generationRows = appendActivity(generationRows, "Agent turn failed: "+errorText)
					}
					if len(pendingPrompts) > 0 {
						generationRows = appendActivity(generationRows, fmt.Sprintf("Cleared %d queued instruction(s) because generation failed.", len(pendingPrompts)))
						pendingPrompts = nil
					}
				} else {
					if generationPurpose == "project" {
						projectPath = update.project
						configPath = update.configPath
						agentSession = update.session
						providerConfig.APIKey = ""
						keyInput.Text = ""
						saveError = ""
						generationRows = appendActivity(generationRows, "Project created: "+projectPath)
						generationRows = appendActivity(generationRows, "Profile saved: "+configPath)
					} else {
						generationRows = appendActivity(generationRows, "Ready for your next instruction.")
					}
				}
				if quitAfterGeneration {
					return nil
				}
				if deferredNavigation != nil {
					action := *deferredNavigation
					deferredNavigation = nil
					navigateWorkspace(action)
					continue
				}
				if len(pendingPrompts) > 0 && agentSession != nil {
					prompt := pendingPrompts[0]
					pendingPrompts = pendingPrompts[1:]
					generationRows = appendActivity(generationRows, "Starting next queued instruction.")
					startAgentTurn(prompt)
				}
				continue
			}
		case result := <-authResults:
			authResults = nil
			applyChatGPTAuthResult(result, &authPending, &authCancel, models, &providerConfig, modelInput, &currentPage, &validationMessage)
			continue
		}
		event = normalizeEscape(event)
		if handled, message := handleAccessModeInput(event.ID, &currentPage, accessReturnPage, list, &config, &providerConfig, agentSession, &generationRows, &pendingPermission); handled {
			validationMessage = message
			continue
		}
		if event.ID == "<F6>" && (currentPage == homePage || currentPage == summaryPage) {
			openAccessMode(&currentPage, &accessReturnPage, &selectedAccessMode, providerConfig, list)
			continue
		}
		if event.ID == "<C-l>" {
			refreshTerminalView()
			lastPage = page(-1)
			continue
		}
		if currentPage == apiKeyPage || currentPage == aiModelPage || currentPage == projectNamePage || currentPage == projectDescriptionPage || currentPage == projectLocationPage || currentPage == repairPathPage {
			if currentPage == aiModelPage {
				var consumed bool
				event, consumed = models.handle(event, modelInput)
				if consumed {
					continue
				}
				if isEscapeKey(event.ID) && previousOptions != nil {
					providerConfig = *previousOptions
					previousOptions, reopening = nil, nil
					currentPage = agentProgressPage
					continue
				}
			}
			if currentPage == apiKeyPage && isEscapeKey(event.ID) {
				currentPage = providerPage
				validationMessage = ""
				continue
			}
			var nextPage page
			advance, quit, message := handleTextInput(event, currentPage, nameInput, descriptionInput, locationInput, keyInput, modelInput, &nextPage)
			if currentPage == aiModelPage && isEscapeKey(event.ID) && providerConfig.Provider == llm.OpenAIWeb {
				nextPage = providerPage
			}
			validationMessage = message
			if quit {
				return nil
			}
			if advance {
				switch currentPage {
				case apiKeyPage:
					if event.ID != "<Enter>" {
						break
					}
					providerConfig.APIKey = keyInput.Text
					if err := saveProviderKey(providerConfig.Provider, keyInput.Text); err != nil {
						validationMessage = "Could not save provider key to YAML: " + err.Error()
						continue
					}
					if config.ProviderKeys == nil {
						config.ProviderKeys = make(map[string]string)
					}
					config.ProviderKeys[string(providerConfig.Provider)] = keyInput.Text
				case aiModelPage:
					if event.ID != "<Enter>" {
						break
					}
					providerConfig.Model = modelInput.Text
					if editingSettings && event.ID == "<Enter>" {
						if err := saveGenerationSelection(&config, providerConfig, reopening); err != nil {
							validationMessage = err.Error()
							continue
						}
						editingSettings = false
						reopening = nil
						currentPage = homePage
						continue
					}
					if reopening == nil {
						if err := saveGenerationSelection(&config, providerConfig, nil); err != nil {
							validationMessage = err.Error()
							continue
						}
					}
				}
				if currentPage == repairPathPage && event.ID == "<Enter>" {
					index := selectedHome - 1
					if index < 0 || index >= len(config.Projects) {
						validationMessage = "Select a saved project to repair."
						continue
					}
					if err := repairProjectDirectory(&config, config.Projects[index].ID, locationInput.Text); err != nil {
						validationMessage = err.Error()
						continue
					}
					if err := projectstore.SaveDefault(config); err != nil {
						validationMessage = err.Error()
						continue
					}
					validationMessage = ""
					currentPage = homePage
					continue
				}
				if currentPage == projectLocationPage && event.ID == "<Enter>" {
					config.ParentDirectory = locationInput.Text
					if err := projectstore.SaveDefault(config); err != nil {
						validationMessage = err.Error()
						continue
					}
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
				if currentPage == aiModelPage && reopening != nil && event.ID == "<Enter>" {
					reopening.AIProvider, reopening.AIModel, reopening.GenerationMode = string(providerConfig.Provider), providerConfig.Model, "agent"
					session, err := resumeProject(*reopening, providerConfig)
					if err != nil {
						validationMessage = err.Error()
						continue
					}
					if err := saveGenerationSelection(&config, providerConfig, reopening); err != nil {
						validationMessage = err.Error()
						continue
					}
					agentSession = session
					draft := workspace.composer.Text
					workspace = newAgentWorkspace()
					workspace.composer.Text = draft
					previousOptions = nil
					workspace.AddUsage(session.Usage())
					agentSecret = providerConfig.APIKey
					generationRows = nil
					for _, row := range session.Transcript() {
						generationRows = appendActivity(generationRows, row)
					}
					generationRows = appendActivity(generationRows, "Session restored. Type your next instruction.")
					currentPage = agentProgressPage
					reopening = nil
					continue
				}
				if currentPage == aiModelPage && nextPage == summaryPage && reopening == nil && providerConfig.Mode == "agent" {
					accessReturnPage = summaryPage
					selectedAccessMode = accessModeIndex(providerConfig.AccessMode)
					list.SelectedRow = selectedAccessMode
					currentPage = accessModePage
					validationMessage = ""
					continue
				}
				currentPage = nextPage
				validationMessage = ""
			}
			continue
		}
		if currentPage == providerPage && isEscapeKey(event.ID) && previousOptions != nil && !authPending {
			providerConfig = *previousOptions
			previousOptions, reopening = nil, nil
			currentPage = agentProgressPage
			continue
		}
		if currentPage == agentProgressPage {
			action := workspace.Handle(event, generationRunning)
			if action.access {
				if generationRunning {
					generationRows = appendActivity(generationRows, "Wait for the current agent action to finish before changing access mode.")
				} else {
					openAccessMode(&currentPage, &accessReturnPage, &selectedAccessMode, providerConfig, list)
				}
				continue
			}
			if action.thinking {
				if providerConfig.Provider != llm.OpenAIWeb {
					generationRows = appendActivity(generationRows, "Thinking effort is available for ChatGPT plan sessions.")
				} else if generationRunning {
					generationRows = appendActivity(generationRows, "Wait for the current response to finish before changing thinking effort.")
				} else {
					previousEffort := providerConfig.ReasoningEffort
					providerConfig.ReasoningEffort = nextReasoningEffort(providerConfig.ReasoningEffort)
					loaded, err := projectstore.LoadDefault()
					if err == nil {
						config = loaded
						var project *projectstore.Project
						project, err = projectForWorkspace(config, projectPath)
						if err == nil {
							err = saveGenerationSelection(&config, providerConfig, project)
						}
					}
					if err != nil {
						providerConfig.ReasoningEffort = previousEffort
						generationRows = appendActivity(generationRows, "Could not save thinking effort: "+err.Error())
					} else {
						agentSession.SetReasoningEffort(providerConfig.ReasoningEffort)
						generationRows = clearThinkingActivity(generationRows)
					}
				}
				continue
			}
			if action.back || action.provider || action.model {
				if generationRunning {
					deferredNavigation = &action
					pendingPrompts = nil
					if generationCancel != nil {
						generationCancel()
					}
					generationRows = appendActivity(generationRows, "Cancellation requested; saving before switching views...")
				} else {
					navigateWorkspace(action)
				}
				continue
			}
			if action.scroll {
				followAgentActivity = action.follow
				continue
			}
			if action.quit {
				if generationRunning {
					if !quitAfterGeneration {
						quitAfterGeneration = true
						if generationCancel != nil {
							generationCancel()
						}
						cancelMessage := "Cancellation requested; waiting for the agent to stop..."
						if generationPurpose == "project" {
							cancelMessage = "Cancellation requested; preserving project and conversation..."
						}
						generationRows = appendActivity(generationRows, cancelMessage)
					}
					continue
				}
				return nil
			}
			if action.prompt != "" {
				if action.queued {
					if len(pendingPrompts) >= maxPendingPrompts {
						workspace.composer.Text = action.prompt
						workspace.composer.Cursor.X = len([]rune(action.prompt))
						generationRows = appendActivity(generationRows, fmt.Sprintf("Prompt queue is full (%d). Send it after an instruction finishes.", maxPendingPrompts))
						continue
					}
					pendingPrompts = append(pendingPrompts, action.prompt)
					generationRows = appendActivity(generationRows, fmt.Sprintf("Instruction queued (%d).", len(pendingPrompts)))
				} else {
					if agentSession == nil {
						generationRows = appendActivity(generationRows, "Wait for the starter generation to finish before sending a prompt.")
						continue
					}

					startAgentTurn(action.prompt)
				}
				followAgentActivity = true
				continue
			}
			continue
		}

		switch event.ID {
		case "q", "<C-c>":
			if currentPage == agentProgressPage && generationRunning {
				if !quitAfterGeneration {
					quitAfterGeneration = true
					if generationCancel != nil {
						generationCancel()
					}
					generationRows = appendActivity(generationRows, "Cancellation requested; preserving project and conversation...")
					if followAgentActivity {
						list.SelectedRow = len(generationRows) - 1
					}
				}
				continue
			}
			return nil
		case "n":
			if currentPage == homePage {
				reopening = nil
				providerConfig = initialGenerationOptions(config)
				selectedMode = generationModeIndex(providerConfig)
				nameInput.Text = ""
				descriptionInput.Text = ""
				wizard = projectWizard{}
				selectedShape, selectedArchitecture, selectedWorkload = 0, 0, workloadIndex(catalog.WorkloadHTTPAPI)
				nameInput.Cursor, descriptionInput.Cursor = 0, 0
				selectedProvider = providerIndex(providerConfig.Provider)
				currentPage = projectNamePage
				continue
			}
			if currentPage == summaryPage {
				currentPage = projectNamePage
			}
		case "d":
			if currentPage == summaryPage {
				currentPage = projectDescriptionPage
			}
		case "p":
			if currentPage == homePage && selectedHome > 0 && selectedHome <= len(config.Projects) {
				locationInput.Title = "Existing project directory"
				locationInput.Placeholder = "Enter the current absolute project folder path"
				locationInput.Text = config.Projects[selectedHome-1].Path
				locationInput.Cursor = utf8.RuneCountInString(locationInput.Text)
				validationMessage = ""
				currentPage = repairPathPage
				continue
			}
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
			if currentPage == homePage {
				editingSettings = true
				providerConfig, reopening = homeProviderSelection(config, selectedHome)
				previousOptions = nil
				selectedProvider = providerIndex(providerConfig.Provider)
				modelInput.Text = providerConfig.Model
				models.invalidate()
				currentPage = providerPage
				continue
			}
			if currentPage == summaryPage {
				providerConfig.Mode = "agent"
				currentPage = providerPage
			}
		case "<Up>", "k":
			if isChoicePage(currentPage) {
				previous := list.SelectedRow
				list.ScrollUp()
				syncChoiceSelection(currentPage, list.SelectedRow, previous, &selectedHome, &selectedMode, &selectedProvider, &selectedShape, &selectedArchitecture, &selectedWorkload, &selectedStack, &selectedAccessMode, &validationMessage)
			} else if currentPage == agentProgressPage {
				list.ScrollUp()
				followAgentActivity = false
			}
		case "<Down>", "j":
			if isChoicePage(currentPage) {
				previous := list.SelectedRow
				list.ScrollDown()
				syncChoiceSelection(currentPage, list.SelectedRow, previous, &selectedHome, &selectedMode, &selectedProvider, &selectedShape, &selectedArchitecture, &selectedWorkload, &selectedStack, &selectedAccessMode, &validationMessage)
			} else if currentPage == agentProgressPage {
				list.ScrollDown()
				followAgentActivity = list.SelectedRow == len(generationRows)-1
			}
		case "<Escape>", "b", "<Backspace>":
			switch currentPage {
			case homePage:
				return nil
			case savedPage:
				config, err = projectstore.LoadDefault()
				if err != nil {
					return err
				}
				currentPage = homePage
			case agentProgressPage:
				if !generationRunning {
					currentPage = summaryPage
				}
			case appShapePage:
				currentPage = summaryPage
			case capabilityPage:
				currentPage = stackPage
			case stackPage:
				currentPage = architecturePage
			case architecturePage:
				if selectedWorkload == workloadIndex(catalog.WorkloadHTTPAPI) {
					currentPage = topologyPage
				} else {
					currentPage = patternPage
				}
			case topologyPage:
				currentPage = patternPage
			case patternPage:
				currentPage = workloadPage
			case workloadPage:
				currentPage = projectLocationPage
			case providerPage:
				if authPending {
					if authCancel != nil {
						authCancel()
					}
					authCancel = nil
					authResults = nil
					authPending = false
					validationMessage = "ChatGPT sign-in cancelled. Choose a provider or press Enter to retry."
				} else {
					currentPage = generationModePage
					if editingSettings {
						editingSettings = false
						reopening = nil
						providerConfig = initialGenerationOptions(config)
						currentPage = homePage
					}
				}
			case generationModePage:
				currentPage = capabilityPage
			case summaryPage:
				currentPage = generationModePage
			}
		case "<Space>":
			if currentPage == patternPage {
				if err := wizard.togglePattern(list.SelectedRow); err != nil {
					validationMessage = err.Error()
				} else {
					validationMessage = ""
				}
			} else if currentPage == capabilityPage {
				if err := wizard.toggleCapability(list.SelectedRow); err != nil {
					validationMessage = err.Error()
				} else {
					validationMessage = ""
				}
			}
		case "<Enter>":
			switch currentPage {
			case homePage:
				if list.SelectedRow == 0 {
					reopening = nil
					providerConfig = initialGenerationOptions(config)
					selectedMode = generationModeIndex(providerConfig)
					nameInput.Text, descriptionInput.Text = "", ""
					wizard = projectWizard{}
					selectedShape, selectedArchitecture, selectedWorkload = 0, 0, workloadIndex(catalog.WorkloadHTTPAPI)
					nameInput.Cursor, descriptionInput.Cursor = 0, 0
					selectedProvider = providerIndex(providerConfig.Provider)
					currentPage = projectNamePage
					continue
				}
				index := list.SelectedRow - 1
				if index < 0 || index >= len(config.Projects) {
					continue
				}
				project := config.Projects[index]
				project = projectstore.NormalizeProject(project)
				wizard.patterns = append([]string(nil), project.PatternIDs...)
				wizard.capabilities = append([]string(nil), project.Capabilities...)
				selectedWorkload = workloadIndex(project.WorkloadID)
				if info, err := os.Stat(project.Path); err != nil || !info.IsDir() {
					validationMessage = "Project directory unavailable: " + project.Path + " | press p to repair its path"
					continue
				}
				validationMessage = ""
				config.LastProjectID = project.ID
				if err := projectstore.SaveDefault(config); err != nil {
					validationMessage = err.Error()
					continue
				}
				reopening = &project
				projectPath = project.Path
				nameInput.Text, descriptionInput.Text = project.Name, project.Description
				locationInput.Text = filepath.Dir(project.Path)
				for i, shape := range catalog.AppShapes() {
					if shape.ID == project.AppShapeID {
						selectedShape = i
						break
					}
				}
				for i, architecture := range catalog.ArchitecturesForShape(project.AppShapeID) {
					if architecture.ID == project.ArchitectureID {
						selectedArchitecture = i
						break
					}
				}
				if project.GenerationMode != "agent" {
					list.SelectedRow = 0
					providerConfig = generationOptions{Mode: "local"}
					configPath, _ = projectstore.DefaultPath()
					currentPage = savedPage
					continue
				}
				providerConfig = generationOptions{Mode: "agent", AccessMode: initialGenerationOptions(config).AccessMode, Provider: llm.ProviderID(project.AIProvider), Model: project.AIModel, ReasoningEffort: llm.ReasoningEffort(project.ReasoningEffort)}
				providerConfig.APIKey = rememberedKey(providerConfig.Provider, config)
				selectedProvider = providerIndex(providerConfig.Provider)
				modelInput.Text = providerConfig.Model
				modelInput.Cursor = utf8.RuneCountInString(modelInput.Text)
				if providerConfig.APIKey == "" && providerConfig.Provider != llm.OpenAIWeb {
					currentPage = providerPage
					continue
				}
				session, err := resumeProject(project, providerConfig)
				if err != nil {
					validationMessage = err.Error()
					continue
				}
				agentSession, agentSecret = session, providerConfig.APIKey
				workspace = newAgentWorkspace()
				generationRows = nil
				workspace.AddUsage(session.Usage())
				for _, row := range session.Transcript() {
					generationRows = appendActivity(generationRows, row)
				}
				generationRows = appendActivity(generationRows, "Session restored. Type your next instruction.")
				reopening = nil
				currentPage = agentProgressPage
			case agentProgressPage:
				if !generationRunning {
					if saveError != "" {
						currentPage = summaryPage
					} else {
						currentPage = savedPage
					}
				}
			case generationModePage:
				selectedMode = list.SelectedRow
				if selectedMode == 0 {
					providerConfig.Mode = "local"
					currentPage = summaryPage
				} else {
					providerConfig.Mode = "agent"
					if providerConfig.APIKey == "" {
						providerConfig.APIKey = rememberedKey(providerConfig.Provider, config)
					}
					if providerConfig.APIKey != "" && providerConfig.Model != "" {
						accessReturnPage = summaryPage
						selectedAccessMode = accessModeIndex(providerConfig.AccessMode)
						list.SelectedRow = selectedAccessMode
						currentPage = accessModePage
					} else {
						currentPage = providerPage
					}
				}
			case providerPage:
				if authPending {
					validationMessage = "Finish or cancel the current sign-in before choosing a provider."
					continue
				}
				selectedProvider = list.SelectedRow
				choice, ok := providerAt(selectedProvider)
				if !ok {
					validationMessage = "Choose a provider to continue."
					continue
				}
				providerChanged := choice.id != providerConfig.Provider
				providerConfig.Provider = choice.id
				providerConfig.APIKey = ""
				providerConfig.Credentials = nil
				if providerChanged {
					providerConfig.Model = ""
					modelInput.Text = ""
					modelInput.Cursor = 0
				}
				if choice.id == llm.OpenAIWeb {
					authCancel, authResults = startChatGPTAuth(choice.newRegistration)
					authPending = true
					validationMessage = ""
				} else {
					keyInput.Text = rememberedKey(choice.id, config)
					keyInput.Cursor = utf8.RuneCountInString(keyInput.Text)
					setFooter(&keyInput.Block, "", false)
					currentPage = apiKeyPage
				}
				validationMessage = ""
			case appShapePage:
				selectedShape = list.SelectedRow
				selectedArchitecture = clampArchitectureIndex(selectedShape, selectedArchitecture)
				currentPage = architecturePage
			case workloadPage:
				selectedWorkload = list.SelectedRow
				selectedShape = indexOfShape(workloadShape(workloadIDAt(selectedWorkload)))
				selectedArchitecture = clampArchitectureIndex(selectedShape, 0)
				list.SelectedRow = 0
				currentPage = patternPage
			case patternPage:
				validationMessage = ""
				if selectedWorkload == workloadIndex(catalog.WorkloadHTTPAPI) {
					list.SelectedRow = topologyIndex(selectedShape)
					currentPage = topologyPage
				} else {
					currentPage = architecturePage
				}
			case topologyPage:
				topologies := catalog.Topologies()
				if list.SelectedRow < 0 || list.SelectedRow >= len(topologies) {
					validationMessage = "Choose a service topology."
					continue
				}
				selectedShape = indexOfShape(topologies[list.SelectedRow].ID)
				selectedArchitecture = clampArchitectureIndex(selectedShape, selectedArchitecture)
				currentPage = architecturePage
			case architecturePage:
				selectedArchitecture = list.SelectedRow
				list.SelectedRow = 0
				currentPage = stackPage
			case stackPage:
				selectedStack = list.SelectedRow
				if !stackSupported(selectedStack) {
					validationMessage = "That stack is planned and cannot be generated yet."
					continue
				}
				list.SelectedRow = 0
				currentPage = capabilityPage
				plannedPath, err := plannedProjectPath(locationInput.Text, nameInput.Text)
				if err != nil {
					validationMessage = err.Error()
					currentPage = projectLocationPage
					continue
				}
				projectPath = plannedPath
				currentPage = generationModePage
			case summaryPage:
				template, ok := templateForChoices(selectedShape, selectedArchitecture)
				if !ok {
					saveError = "Select a valid shape and architecture before saving."
					continue
				}
				if providerConfig.Mode == "agent" {
					jobContext, cancel := context.WithCancel(context.Background())
					generationCancel = cancel
					generationUpdates = make(chan generationUpdate, 32)
					updates := generationUpdates
					generationRows = nil
					generationRows = appendActivity(generationRows, "Preparing project: "+nameInput.Text)
					generationRows = appendActivity(generationRows, "Blueprint: "+template.Name)
					generationRows = appendActivity(generationRows, "Provider: "+generationLabel(providerConfig))
					generationRows = appendActivity(generationRows, "Workspace: "+projectPath)
					followAgentActivity = true
					list.SelectedRow = len(generationRows) - 1
					generationRunning = true
					generationPurpose = "project"
					quitAfterGeneration = false
					agentSecret = providerConfig.APIKey
					currentPage = agentProgressPage
					projectName, description, targetPath, options := nameInput.Text, descriptionInput.Text, projectPath, providerConfig
					profile := projectWizard{patterns: append([]string(nil), wizard.patterns...), capabilities: append([]string(nil), wizard.capabilities...)}
					go func() {
						reporter := &generationReporter{ctx: jobContext, updates: updates, secret: options.APIKey}
						savedPath, savedConfig, err := saveProjectWithProfileProgress(jobContext, projectName, description, targetPath, template, options, profile, reporter)
						updates <- generationUpdate{project: savedPath, configPath: savedConfig, session: reporter.AgentSession(), err: err, done: true}
					}()
					continue
				}
				savedPath, savedConfig, err := saveProjectWithProfileProgress(context.Background(), nameInput.Text, descriptionInput.Text, projectPath, template, providerConfig, wizard, io.Discard)
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
