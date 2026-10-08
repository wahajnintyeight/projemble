package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
	"projemble/internal/projectstore"
)

func renderProjectManagementPage(list *widgets.List, project *projectstore.Project, validation string, width, height int) {
	if project == nil {
		updateChoiceList(list, "Project", nil, 0, width, height)
		setFooter(&list.Block, "Esc/b projects", false)
		ui.Render(list)
		return
	}
	updateChoiceList(list, "Project / "+project.Name, []catalogChoice{
		{name: "Add AI agent", description: "Connect a provider so an agent can work on this scaffold."},
		{name: "Remove from Projemble", description: "Remove the saved profile and keep every project file."},
		{name: "Delete project", description: "Permanently remove the saved profile and project folder."},
	}, list.SelectedRow, width, height)
	setFooter(&list.Block, "Enter choose | Esc/b projects", false)
	if validation != "" {
		setFooter(&list.Block, validation, true)
	}
	text := styleLabel("Scaffold ready") + " | " + project.StackID + " | " + project.Path +
		"\n\nThis project was created without an AI agent. Add one when you want Projemble to explore, change, and build on it."
	renderOnboardingChoices(list, text, width, height)
}

func renderProjectDeletePage(list *widgets.List, project *projectstore.Project, validation string, width, height int) {
	if project == nil {
		updateChoiceList(list, "Delete project", nil, 0, width, height)
		setFooter(&list.Block, "Esc/b back", false)
		ui.Render(list)
		return
	}
	choices := []catalogChoice{
		{name: "Cancel", description: "Return to project options."},
		{name: "Remove profile only", description: "Keep the project directory and all files."},
		{name: "Delete project folder and profile", description: "Permanently delete all files in the project directory."},
	}
	updateChoiceList(list, "Delete / "+project.Name, choices, list.SelectedRow, width, height)
	setFooter(&list.Block, "Enter confirm | Esc/b cancel", false)
	if validation != "" {
		setFooter(&list.Block, validation, true)
	}
	text := styleLabel("Choose what to remove.") + "\n\nProject folder:\n" + project.Path +
		"\n\nDeleting the folder cannot be undone. The folder is checked before removal."
	renderOnboardingChoices(list, text, width, height)
}

func reloadManagedProject(config *projectstore.Config, path string) (*projectstore.Project, error) {
	loaded, err := projectstore.LoadDefault()
	if err != nil {
		return nil, err
	}
	project, err := projectForWorkspace(loaded, path)
	if err != nil {
		return nil, err
	}
	*config = loaded
	return project, nil
}

func projectExists(config projectstore.Config, id string) bool {
	for _, project := range config.Projects {
		if project.ID == id {
			return true
		}
	}
	return false
}

func removeManagedProject(config *projectstore.Config, project projectstore.Project, deleteFiles bool) error {
	path := ""
	if deleteFiles {
		var err error
		path, err = safeProjectDeletionPath(project.Path)
		if err != nil {
			return err
		}
	}
	next := *config
	next.Projects = make([]projectstore.Project, 0, len(config.Projects))
	found := false
	for _, saved := range config.Projects {
		if saved.ID == project.ID {
			found = true
			continue
		}
		next.Projects = append(next.Projects, saved)
	}
	if !found {
		return fmt.Errorf("project profile %q is no longer saved", project.Name)
	}
	if next.LastProjectID == project.ID {
		next.LastProjectID = ""
	}
	if err := projectstore.SaveDefault(next); err != nil {
		return err
	}
	*config = next
	if deleteFiles {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("profile removed, but could not delete project folder %q: %w", path, err)
		}
	}
	return nil
}

func handleProjectManagementEnter(current page, next *page, history *pageHistory, config *projectstore.Config, project **projectstore.Project, options *generationOptions, selectedProvider *int, modelInput *widgets.Input, models *modelPicker, selectedHome *int, validation *string, list *widgets.List) bool {
	switch current {
	case projectManagePage:
		if *project == nil {
			*validation = "Project profile unavailable. Return to Projects and reload it."
			return true
		}
		if list.SelectedRow == 0 {
			*options = initialGenerationOptions(*config)
			options.Mode = "agent"
			options.APIKey = rememberedKey(options.Provider, *config)
			*selectedProvider = providerIndex(options.Provider)
			list.SelectedRow = *selectedProvider
			modelInput.Text = options.Model
			modelInput.Cursor = utf8.RuneCountInString(modelInput.Text)
			models.invalidate()
			*next = providerPage
		} else if list.SelectedRow == 1 || list.SelectedRow == 2 {
			*validation = ""
			list.SelectedRow = 0
			*next = projectDeletePage
		}
		return true
	case projectDeletePage:
		if list.SelectedRow == 0 {
			history.ReturnTo(projectManagePage)
			*next = projectManagePage
			return true
		}
		if *project == nil {
			*validation = "Project profile unavailable. Return to Projects and reload it."
			return true
		}
		if err := removeManagedProject(config, **project, list.SelectedRow == 2); err != nil {
			*validation = err.Error()
			if !projectExists(*config, (**project).ID) {
				*project = nil
				*selectedHome = 0
				*next = homePage
				history.Reset(homePage)
			}
			return true
		}
		*project = nil
		*selectedHome = 0
		*validation = "Project removed. Project files were kept."
		if list.SelectedRow == 2 {
			*validation = "Project and its files were deleted."
		}
		*next = homePage
		history.Reset(homePage)
		return true
	default:
		return false
	}
}

func safeProjectDeletionPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("refusing to delete project path that is not absolute: %q", path)
	}
	clean := filepath.Clean(path)
	if filepath.Dir(clean) == clean {
		return "", fmt.Errorf("refusing to delete a filesystem root: %q", clean)
	}
	info, err := os.Lstat(clean)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("refusing to delete a project path that is a symbolic link: %q", clean)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect project folder %q: %w", clean, err)
	}
	return clean, nil
}
