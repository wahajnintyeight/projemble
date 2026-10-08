package tui

import (
	"fmt"

	"projemble/internal/catalog"
)

type projectWizard struct {
	patterns     []string
	capabilities []string
}

func workloadChoices() []catalogChoice {
	items := catalog.Workloads()
	choices := make([]catalogChoice, 0, len(items))
	for _, item := range items {
		choices = append(choices, catalogChoice{name: item.Name, description: item.Description})
	}
	return choices
}

func stackChoices() []catalogChoice {
	setting, _ := catalog.SettingByID(catalog.SettingStack)
	choices := make([]catalogChoice, 0, len(setting.Options))
	for _, option := range setting.Options {
		name := option.Name + " (planned)"
		if option.Supported {
			name = option.Name + " (supported)"
		}
		choices = append(choices, catalogChoice{name: name, description: option.Description})
	}
	return choices
}

func stackSupported(index int) bool {
	setting, _ := catalog.SettingByID(catalog.SettingStack)
	return index >= 0 && index < len(setting.Options) && setting.Options[index].Supported
}

func workloadShape(id string) string {
	switch id {
	case catalog.WorkloadHTTPAPI:
		return catalog.ShapeMonolith
	case catalog.WorkloadOneShot:
		return catalog.ShapeOneShotJob
	case catalog.WorkloadCLI:
		return catalog.ShapeCLI
	case catalog.WorkloadWorker:
		return catalog.ShapeWorker
	case catalog.WorkloadLibrary:
		return catalog.ShapeLibrary
	default:
		return ""
	}
}

func workloadIDAt(index int) string {
	items := catalog.Workloads()
	if index < 0 || index >= len(items) {
		return ""
	}
	return items[index].ID
}

func indexOfShape(id string) int {
	for index, item := range catalog.AppShapes() {
		if item.ID == id {
			return index
		}
	}
	return 0
}

func topologyChoices() []catalogChoice {
	items := catalog.Topologies()
	choices := make([]catalogChoice, 0, len(items))
	for _, item := range items {
		choices = append(choices, catalogChoice{name: item.Name, description: item.Description})
	}
	return choices
}

func patternChoices(selected []string) []catalogChoice {
	items := catalog.Patterns()
	choices := make([]catalogChoice, 0, len(items))
	for _, item := range items {
		marker := "[ ] "
		if contains(selected, item.ID) {
			marker = "[✓] "
		}
		choices = append(choices, catalogChoice{name: marker + item.Name, description: item.Description})
	}
	return choices
}

func capabilityChoices(selected []string) []catalogChoice {
	items := catalog.Capabilities()
	choices := make([]catalogChoice, 0, len(items))
	for _, item := range items {
		marker := "[planned] "
		if item.Supported {
			marker = "[ ] "
			if contains(selected, item.ID) {
				marker = "[✓] "
			}
		}
		choices = append(choices, catalogChoice{name: marker + item.Name, description: item.Category + " - " + item.Description})
	}
	return choices
}

func (wizard *projectWizard) togglePattern(index int) error {
	items := catalog.Patterns()
	if index < 0 || index >= len(items) {
		return fmt.Errorf("choose an application pattern")
	}
	wizard.patterns = toggle(wizard.patterns, items[index].ID)
	return nil
}

func (wizard *projectWizard) toggleCapability(index int) error {
	items := catalog.Capabilities()
	if index < 0 || index >= len(items) {
		return fmt.Errorf("choose a capability")
	}
	item := items[index]
	if !item.Supported {
		return fmt.Errorf("%s is planned and cannot be generated yet", item.Name)
	}
	if item.Category == "Primary database" && !contains(wizard.capabilities, item.ID) {
		for _, existing := range wizard.capabilities {
			if old, ok := catalog.CapabilityByID(existing); ok && old.Category == item.Category {
				wizard.capabilities = remove(wizard.capabilities, existing)
			}
		}
	}
	wizard.capabilities = toggle(wizard.capabilities, item.ID)
	return nil
}

func toggle(values []string, value string) []string {
	if contains(values, value) {
		return remove(values, value)
	}
	return append(values, value)
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func remove(values []string, value string) []string {
	result := values[:0]
	for _, item := range values {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}
