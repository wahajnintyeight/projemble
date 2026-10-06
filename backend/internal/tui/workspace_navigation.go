package tui

import (
	"fmt"
	"projemble/internal/projectstore"
)

func workspaceDestination(action workspaceAction) page {
	if action.provider {
		return providerPage
	}
	if action.model {
		return aiModelPage
	}
	return homePage
}

func projectForWorkspace(config projectstore.Config, path string) (*projectstore.Project, error) {
	for _, project := range config.Projects {
		if project.Path == path {
			return &project, nil
		}
	}
	return nil, fmt.Errorf("project profile is unavailable; open Projects to select a saved project")
}
