package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"projemble/internal/agent"
	"projemble/internal/auth"
	"projemble/internal/catalog"
	"projemble/internal/llm"
	"projemble/internal/projectstore"
)

func homeChoices(config projectstore.Config) []catalogChoice {
	rows := []catalogChoice{{name: "Create a project", description: "Define your idea, choose a blueprint, and generate its scaffold"}}
	for _, project := range config.Projects {
		status := project.Status
		if status == "" {
			status = "saved"
		}
		template, _ := catalog.TemplateByID(project.TemplateID)
		rows = append(rows, catalogChoice{name: project.Name, description: fmt.Sprintf("%s | %s | %s", template.Name, status, project.Path)})
	}
	return rows
}

func rememberedDirectory(config projectstore.Config) string {
	if config.ParentDirectory != "" {
		return config.ParentDirectory
	}
	if len(config.Projects) > 0 {
		return filepath.Dir(config.Projects[len(config.Projects)-1].Path)
	}
	return ""
}

func repairProjectDirectory(config *projectstore.Config, projectID, path string) error {
	absolutePath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("resolve project directory: %w", err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return fmt.Errorf("open project directory %q: %w", absolutePath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("project path is not a directory: %s", absolutePath)
	}
	for _, project := range config.Projects {
		if project.ID == projectID {
			project.Path = absolutePath
			if err := config.Upsert(project); err != nil {
				return err
			}
			config.LastProjectID = project.ID
			return nil
		}
	}
	return fmt.Errorf("saved project %q was not found", projectID)
}

func rememberedKey(provider llm.ProviderID, config projectstore.Config) string {
	if key := config.ProviderKeys[string(provider)]; key != "" {
		return key
	}
	for _, choice := range providerCatalog() {
		if choice.id == provider && choice.env != "" {
			if key := os.Getenv(choice.env); key != "" {
				return key
			}
			return ""
		}
	}
	return ""
}

func saveProviderKey(provider llm.ProviderID, key string) error {
	config, err := projectstore.LoadDefault()
	if err != nil {
		return err
	}
	if config.ProviderKeys == nil {
		config.ProviderKeys = make(map[string]string)
	}
	config.ProviderKeys[string(provider)] = key
	return projectstore.SaveDefault(config)
}

func updateProjectStatus(path, status string) error {
	config, err := projectstore.LoadDefault()
	if err != nil {
		return err
	}
	for _, project := range config.Projects {
		if project.Path == path {
			project.Status = status
			if err := config.Upsert(project); err != nil {
				return err
			}
			return projectstore.SaveDefault(config)
		}
	}
	return nil
}

func resumeProject(project projectstore.Project, options generationOptions) (*agent.Agent, error) {
	info, err := os.Stat(project.Path)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("project directory is unavailable: %s", project.Path)
	}
	if options.Provider == llm.OpenAIWeb {
		options.Credentials = &auth.ChatGPTTokenSource{}
	}
	session, err := agent.New(agent.Config{ProviderID: options.Provider, Model: options.Model, ReasoningEffort: options.ReasoningEffort, APIKey: options.APIKey, Credentials: options.Credentials})
	if err != nil {
		return nil, err
	}
	if err := session.Restore(project.Path); err != nil {
		return nil, err
	}
	return session, nil
}
