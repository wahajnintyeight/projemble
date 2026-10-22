package catalog

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type OptionStatus string

const (
	StatusRecommended  OptionStatus = "recommended"
	StatusSupported    OptionStatus = "supported"
	StatusExperimental OptionStatus = "experimental"
	StatusPlanned      OptionStatus = "planned"
	StatusLegacy       OptionStatus = "legacy"
)

const (
	StepServiceFramework = "service-framework"
	StepDatabase         = "database"

	ContextWorkload = "workload"
	ContextTopology = "topology"

	OptionNone = "none"
)

type Selection map[string]string

type Option struct {
	ID        string
	Name      string
	Summary   string
	Status    OptionStatus
	Modules   []string
	Needs     []string
	Requires  []string
	Conflicts []string
	Replaces  []string
	Notes     string
}

func (option Option) Selectable() bool {
	return option.Status != StatusPlanned && option.Status != StatusLegacy
}

type Step struct {
	ID        string
	Name      string
	Question  string
	AppliesTo func(Selection) bool
	Default   func(Selection) string
	Options   []Option
}

func (step Step) applies(selection Selection) bool {
	return step.AppliesTo == nil || step.AppliesTo(selection)
}

func (step Step) option(id string) (Option, bool) {
	for _, option := range step.Options {
		if option.ID == id {
			return option, true
		}
	}
	return Option{}, false
}

func (step Step) defaultOption(selection Selection) string {
	if step.Default != nil {
		return step.Default(selection)
	}
	for _, option := range step.Options {
		if option.Status == StatusRecommended {
			return option.ID
		}
	}
	return OptionNone
}

type Plan struct {
	Preset    string
	Choices   map[string]string
	Options   []Option
	Modules   []ModuleInfo
	GoVersion string
	Needs     []string
}

func Resolve(selection Selection, preset string) (Plan, error) {
	var problems []error
	chosen := Selection{}
	for key, value := range selection {
		chosen[key] = value
	}
	presetChoices := map[string]string{}
	if preset != "" {
		found, ok := PresetByID(preset)
		if !ok {
			problems = append(problems, fmt.Errorf("unknown preset %q", preset))
		} else {
			presetChoices = found.Choices
		}
	}
	known := map[string]bool{ContextWorkload: true, ContextTopology: true}
	for _, step := range steps {
		known[step.ID] = true
	}
	for key := range selection {
		if !known[key] {
			problems = append(problems, fmt.Errorf("unknown step %q", key))
		}
	}
	plan := Plan{Preset: preset, Choices: map[string]string{}}
	var resolved []Step
	for _, step := range steps {
		if !step.applies(chosen) {
			if value := selection[step.ID]; value != "" && value != OptionNone {
				problems = append(problems, fmt.Errorf("%s does not apply to this project", step.Name))
			}
			continue
		}
		id := selection[step.ID]
		if id == "" {
			id = presetChoices[step.ID]
		}
		if id == "" {
			id = step.defaultOption(chosen)
		}
		chosen[step.ID] = id
		plan.Choices[step.ID] = id
		resolved = append(resolved, step)
	}
	replaced := map[string]string{}
	seenModules := map[string]bool{}
	for _, step := range resolved {
		option, ok := step.option(plan.Choices[step.ID])
		if !ok {
			problems = append(problems, fmt.Errorf("%s: unknown option %q", step.Name, plan.Choices[step.ID]))
			continue
		}
		if !option.Selectable() {
			problems = append(problems, fmt.Errorf("%s: %s is %s and cannot be selected yet", step.Name, option.Name, option.Status))
			continue
		}
		for _, requirement := range option.Requires {
			if !satisfied(chosen, requirement) {
				problems = append(problems, fmt.Errorf("%s: %s requires %s", step.Name, option.Name, requirement))
			}
		}
		for _, conflict := range option.Conflicts {
			if satisfied(chosen, conflict) {
				problems = append(problems, fmt.Errorf("%s: %s conflicts with %s", step.Name, option.Name, conflict))
			}
		}
		for _, target := range option.Replaces {
			replaced[target] = step.ID
		}
		plan.Options = append(plan.Options, option)
		plan.Needs = appendUnique(plan.Needs, option.Needs...)
		for _, path := range option.Modules {
			module, ok := ModuleByPath(path)
			if !ok {
				problems = append(problems, fmt.Errorf("%s: module %s is not in the registry", option.Name, path))
				continue
			}
			if !seenModules[path] {
				seenModules[path] = true
				plan.Modules = append(plan.Modules, module)
			}
		}
	}
	for target, owner := range replaced {
		if value := selection[target]; value != "" && value != OptionNone {
			problems = append(problems, fmt.Errorf("%s is handled by %s; remove the %s choice", target, owner, target))
		}
	}
	plan.GoVersion = GoDirective(plan.Modules)
	sort.Strings(plan.Needs)
	if len(problems) > 0 {
		return Plan{}, errors.Join(problems...)
	}
	return plan, nil
}

func satisfied(selection Selection, constraint string) bool {
	step, option, found := strings.Cut(constraint, "=")
	return found && selection[step] == option
}

func appendUnique(list []string, values ...string) []string {
	for _, value := range values {
		exists := false
		for _, item := range list {
			if item == value {
				exists = true
				break
			}
		}
		if !exists {
			list = append(list, value)
		}
	}
	return list
}
