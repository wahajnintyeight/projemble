package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"projemble/internal/agent"
	"projemble/internal/catalog"
	"projemble/internal/generator"
	"projemble/internal/llm"
	"projemble/internal/projectstore"
)

func saveProject(name, description, path string, template catalog.Template) (string, string, error) {
	return saveProjectWithGeneration(name, description, path, template, generationOptions{Mode: "local"})
}

func saveProjectWithGeneration(name, description, path string, template catalog.Template, options generationOptions) (string, string, error) {
	return saveProjectWithProgress(context.Background(), name, description, path, template, options, io.Discard)
}

func saveProjectWithProgress(parent context.Context, name, description, path string, template catalog.Template, options generationOptions, progress io.Writer) (string, string, error) {
	return saveProjectWithProfileProgress(parent, name, description, path, template, options, projectWizard{}, progress)
}

func saveProjectWithProfileProgress(parent context.Context, name, description, path string, template catalog.Template, options generationOptions, wizard projectWizard, progress io.Writer) (string, string, error) {
	if err := parent.Err(); err != nil {
		return "", "", err
	}
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
	accessMode := agent.AccessMode(options.AccessMode)
	if !accessMode.Valid() {
		accessMode = agent.AccessAskAlways
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
	config.ParentDirectory = filepath.Dir(path)
	config.AgentAccessMode = string(accessMode)
	config.Generation = projectstore.GenerationDefaults{Mode: options.Mode, Provider: string(options.Provider), Model: options.Model, ReasoningEffort: string(options.ReasoningEffort)}
	settings := make(map[string]string)
	if template.ServiceFrameworkID != "" {
		settings[catalog.SettingServiceFramework] = template.ServiceFrameworkID
	}
	project := projectstore.Project{
		Name:            name,
		Description:     description,
		Path:            path,
		StackID:         template.StackID,
		AppShapeID:      template.AppShapeID,
		ArchitectureID:  template.ArchitectureID,
		TemplateID:      template.ID,
		PatternIDs:      append([]string(nil), wizard.patterns...),
		Capabilities:    append([]string(nil), wizard.capabilities...),
		GenerationMode:  options.Mode,
		AIProvider:      string(options.Provider),
		AIModel:         options.Model,
		ReasoningEffort: string(options.ReasoningEffort),
		Settings:        settings,
	}
	if err := config.Upsert(project); err != nil {
		return "", "", err
	}
	if err := reportActivity(progress, "Creating project directory: "+path); err != nil {
		return "", "", err
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		if os.IsExist(err) {
			return "", "", fmt.Errorf("project directory already exists: %s", path)
		}
		return "", "", fmt.Errorf("create project directory %q: %w", path, err)
	}
	if err := generator.Generate(project); err != nil {
		return "", "", cleanupCreatedProject(path, "generate project files", err)
	}
	if err := reportActivity(progress, "Starter template created."); err != nil {
		return "", "", fmt.Errorf("report generation progress: %w", err)
	}
	project.Status = "ready"
	if options.Mode == "agent" {
		project.Status = "interrupted"
	}
	// Save the scaffold before network work so failure cannot erase the project.
	for i := range config.Projects {
		if config.Projects[i].Path == path {
			config.Projects[i].Status = project.Status
			config.LastProjectID = config.Projects[i].ID
		}
	}
	if err := projectstore.SaveDefault(config); err != nil {
		return path, "", fmt.Errorf("save project profile: %w", err)
	}
	if options.Mode == "agent" {
		approver, _ := progress.(agent.PermissionApprover)
		providerAgent, err := agent.New(agent.Config{ProviderID: options.Provider, Model: options.Model, ReasoningEffort: options.ReasoningEffort, AccessMode: accessMode, Approver: approver, APIKey: options.APIKey, Credentials: options.Credentials, Profile: agentProfile(project)})
		if err != nil {
			return "", "", fmt.Errorf("configure AI agent: %w (starter preserved at %s)", err, path)
		}
		if reporter, ok := progress.(interface{ SetAgentSession(*agent.Agent) }); ok {
			reporter.SetAgentSession(providerAgent)
		}
		ctx, cancel := context.WithTimeout(parent, 12*time.Minute)
		err = providerAgent.Run(ctx, path, generationTask(project, template), progress)
		cancel()
		if err != nil {
			return "", "", fmt.Errorf("AI generation failed: %w (project preserved at %s)", err, path)
		}
	}
	if err := reportActivity(progress, "Saving project profile to local YAML."); err != nil {
		return "", "", fmt.Errorf("report generation progress: %w", err)
	}
	if err := parent.Err(); err != nil {
		return "", "", fmt.Errorf("AI generation cancelled: %w", err)
	}
	for i := range config.Projects {
		if config.Projects[i].Path == path {
			config.Projects[i].Status = "ready"
		}
	}
	if err := projectstore.SaveDefault(config); err != nil {
		return "", "", fmt.Errorf("save project profile: %w", err)
	}
	configPath, err := projectstore.DefaultPath()
	if err != nil {
		return "", "", err
	}
	return path, configPath, nil
}

func reportActivity(output io.Writer, message string) error {
	if output == nil {
		return nil
	}
	_, err := fmt.Fprintln(output, message)
	return err
}

func cleanupCreatedProject(path, stage string, cause error) error {
	if cleanupErr := os.RemoveAll(path); cleanupErr != nil {
		return fmt.Errorf("%s: %w (could not clean newly created project directory: %v)", stage, cause, cleanupErr)
	}
	return fmt.Errorf("%s: %w", stage, cause)
}

func generationTask(project projectstore.Project, template catalog.Template) string {
	return fmt.Sprintf(`Implement a useful first version of this project inside the existing starter workspace.

Project: %s
Purpose: %s
Stack: %s
Application shape: %s
		Workload: %s
Topology: %s
Architecture: %s
Patterns: %s
Capabilities: %s
Template: %s

Keep the selected profile and starter structure. Add only the smallest coherent domain functionality that serves the stated purpose. Keep the code buildable, add focused tests, and run the available checks before finishing. Do not add external services, credentials, or dependencies unless the described purpose requires them.`, project.Name, project.Description, template.StackID, template.AppShapeID, project.WorkloadID, project.TopologyID, template.ArchitectureID, strings.Join(project.PatternIDs, ", "), strings.Join(project.Capabilities, ", "), template.Name)
}
