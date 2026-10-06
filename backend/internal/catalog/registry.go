package catalog

var appShapesByID = map[string]AppShape{
	ShapeMonolith: {
		ID:          ShapeMonolith,
		Name:        "Monolith",
		Description: "One deployable Go application",
	},
	ShapeMicroservices: {
		ID:          ShapeMicroservices,
		Name:        "Microservices (go-micro)",
		Description: "Separate Go services connected with go-micro",
	},
}

var architecturesByID = map[string]Architecture{
	ArchitectureLayered: {
		ID:          ArchitectureLayered,
		Name:        "Layered",
		Description: "Pragmatic handler, service, and repository layers",
	},
	ArchitectureClean: {
		ID:          ArchitectureClean,
		Name:        "Clean / Hexagonal",
		Description: "Use cases at the center; infrastructure behind ports",
	},
	ArchitectureDDD: {
		ID:          ArchitectureDDD,
		Name:        "Domain-Driven Design",
		Description: "Bounded contexts and explicit domain models",
	},
}

var templatesByID = map[string]Template{
	"go-monolith-layered": newTemplate(
		"go-monolith-layered", "Go Monolith - Layered",
		"HTTP API starter with handler, service, and repository layers",
		ShapeMonolith, ArchitectureLayered, "",
	),
	"go-monolith-clean-hexagonal": newTemplate(
		"go-monolith-clean-hexagonal", "Go Monolith - Clean / Hexagonal",
		"HTTP API starter with use cases, ports, and adapters",
		ShapeMonolith, ArchitectureClean, "",
	),
	"go-monolith-ddd": newTemplate(
		"go-monolith-ddd", "Go Monolith - Domain-Driven Design",
		"HTTP API starter organized around bounded contexts",
		ShapeMonolith, ArchitectureDDD, "",
	),
	"go-microservices-layered": newTemplate(
		"go-microservices-layered", "Go Microservices - Layered",
		"go-micro service starter with handler, service, and repository layers",
		ShapeMicroservices, ArchitectureLayered, "go-micro",
	),
	"go-microservices-clean-hexagonal": newTemplate(
		"go-microservices-clean-hexagonal", "Go Microservices - Clean / Hexagonal",
		"go-micro service starter with use cases, ports, and adapters",
		ShapeMicroservices, ArchitectureClean, "go-micro",
	),
	"go-microservices-ddd": newTemplate(
		"go-microservices-ddd", "Go Microservices - Domain-Driven Design",
		"go-micro services organized around bounded contexts",
		ShapeMicroservices, ArchitectureDDD, "go-micro",
	),
}

var settingsByID = map[string]Setting{
	SettingStack: {
		ID:   SettingStack,
		Name: "Language and stack",
		Options: []SettingOption{{
			ID:          StackGo,
			Name:        "Go",
			Description: "Go application and module",
		}},
	},
	SettingServiceFramework: {
		ID:   SettingServiceFramework,
		Name: "Service framework",
		Options: []SettingOption{{
			ID:          "go-micro",
			Name:        "go-micro",
			Description: "Go framework for microservice discovery and communication",
		}},
	},
}

func newTemplate(id, name, description, shapeID, architectureID, serviceFrameworkID string) Template {
	return Template{
		ID:                 id,
		Name:               name,
		Description:        description,
		StackID:            StackGo,
		AppShapeID:         shapeID,
		ArchitectureID:     architectureID,
		ServiceFrameworkID: serviceFrameworkID,
	}
}

func AppShapes() []AppShape {
	return []AppShape{appShapesByID[ShapeMonolith], appShapesByID[ShapeMicroservices]}
}

func AppShapeByID(id string) (AppShape, bool) {
	shape, ok := appShapesByID[id]
	return shape, ok
}

func Architectures() []Architecture {
	return []Architecture{
		architecturesByID[ArchitectureLayered],
		architecturesByID[ArchitectureClean],
		architecturesByID[ArchitectureDDD],
	}
}

func ArchitectureByID(id string) (Architecture, bool) {
	architecture, ok := architecturesByID[id]
	return architecture, ok
}

func Templates() []Template {
	return []Template{
		templatesByID["go-monolith-layered"],
		templatesByID["go-monolith-clean-hexagonal"],
		templatesByID["go-monolith-ddd"],
		templatesByID["go-microservices-layered"],
		templatesByID["go-microservices-clean-hexagonal"],
		templatesByID["go-microservices-ddd"],
	}
}

func TemplateByID(id string) (Template, bool) {
	template, ok := templatesByID[id]
	return template, ok
}

func Settings() []Setting {
	settings := []Setting{settingsByID[SettingStack], settingsByID[SettingServiceFramework]}
	for i := range settings {
		settings[i].Options = append([]SettingOption(nil), settings[i].Options...)
	}
	return settings
}

func SettingByID(id string) (Setting, bool) {
	setting, ok := settingsByID[id]
	if !ok {
		return Setting{}, false
	}
	setting.Options = append([]SettingOption(nil), setting.Options...)
	return setting, true
}
