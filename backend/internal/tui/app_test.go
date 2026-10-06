package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

func TestWizardInputValidationAndNavigation(t *testing.T) {
	nameInput, descriptionInput := widgets.NewInput(), widgets.NewInput()
	locationInput := widgets.NewInput()
	var nextPage page

	advance, quit, message := handleTextInput(ui.Event{ID: "<Enter>"}, projectNamePage, nameInput, descriptionInput, locationInput, &nextPage)
	if advance || quit || message == "" {
		t.Fatalf("empty project name: advance=%v quit=%v message=%q", advance, quit, message)
	}
	for _, r := range "CON" {
		nameInput.InsertRune(r)
	}
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectNamePage, nameInput, descriptionInput, locationInput, &nextPage)
	if advance || quit || message == "" {
		t.Fatalf("reserved Windows project name accepted: advance=%v quit=%v message=%q", advance, quit, message)
	}
	nameInput.Text = "inventory-api"
	nameInput.Cursor = len(nameInput.Text)
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectNamePage, nameInput, descriptionInput, locationInput, &nextPage)
	if !advance || quit || message != "" || nextPage != projectDescriptionPage {
		t.Fatalf("valid project name: advance=%v quit=%v message=%q next=%v", advance, quit, message, nextPage)
	}
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectDescriptionPage, nameInput, descriptionInput, locationInput, &nextPage)
	if advance || quit || message == "" {
		t.Fatalf("empty description: advance=%v quit=%v message=%q", advance, quit, message)
	}
	descriptionInput.Text = "Tracks stock levels"
	descriptionInput.Cursor = len(descriptionInput.Text)
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectDescriptionPage, nameInput, descriptionInput, locationInput, &nextPage)
	if !advance || quit || message != "" || nextPage != projectLocationPage {
		t.Fatalf("valid description: advance=%v quit=%v message=%q next=%v", advance, quit, message, nextPage)
	}
	locationInput.Text = t.TempDir()
	advance, quit, message = handleTextInput(ui.Event{ID: "<Enter>"}, projectLocationPage, nameInput, descriptionInput, locationInput, &nextPage)
	if !advance || quit || message != "" || nextPage != appShapePage {
		t.Fatalf("valid project location: advance=%v quit=%v message=%q next=%v", advance, quit, message, nextPage)
	}
}

func TestProjectLocationValidationAndPlannedPath(t *testing.T) {
	parent := t.TempDir()
	gotParent, err := validateProjectParent(parent)
	if err != nil || gotParent != filepath.Clean(parent) {
		t.Fatalf("existing absolute directory rejected: parent=%q err=%v", gotParent, err)
	}
	if _, err := validateProjectParent("relative/path"); err == nil {
		t.Fatal("relative project location was accepted")
	}
	missing := filepath.Join(parent, "does-not-exist")
	if _, err := validateProjectParent(missing); err == nil {
		t.Fatal("nonexistent project location was accepted")
	}
	file := filepath.Join(parent, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := validateProjectParent(file); err == nil {
		t.Fatal("file was accepted as project location")
	}
	planned, err := plannedProjectPath(parent, "sample-app")
	if err != nil || planned != filepath.Join(parent, "sample-app") {
		t.Fatalf("planned path=%q err=%v", planned, err)
	}
	if err := os.Mkdir(planned, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := plannedProjectPath(parent, "sample-app"); err == nil {
		t.Fatal("existing project directory was accepted")
	}
}

func TestSaveWizardProfilesForEveryTemplate(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("APPDATA", configRoot)
	parent := t.TempDir()

	for i, template := range catalog.Templates() {
		name := fmt.Sprintf("sample-%d", i)
		path, err := plannedProjectPath(parent, name)
		if err != nil {
			t.Fatalf("plan path for template %s: %v", template.ID, err)
		}
		if _, _, err := saveProject(name, "A test project", path, template); err != nil {
			t.Fatalf("save template %s: %v", template.ID, err)
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("project directory was not created: path=%q info=%v err=%v", path, info, err)
		}
	}

	configPath, err := projectstore.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	config, err := projectstore.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(config.Projects), len(catalog.Templates()); got != want {
		t.Fatalf("saved project count = %d, want %d", got, want)
	}
	for i, template := range catalog.Templates() {
		project := config.Projects[i]
		if project.Name != fmt.Sprintf("sample-%d", i) || project.Description != "A test project" || project.TemplateID != template.ID {
			t.Errorf("saved project %d mismatch: %+v", i, project)
		}
		if template.ServiceFrameworkID != "" && project.Settings[catalog.SettingServiceFramework] != template.ServiceFrameworkID {
			t.Errorf("template %s lost service framework setting: %v", template.ID, project.Settings)
		}
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("YAML config was not created: %v", err)
	}
}
