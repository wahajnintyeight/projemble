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
	ShapeOneShotJob: {
		ID:          ShapeOneShotJob,
		Name:        "One-shot job",
		Description: "Run a Go task once and exit: scraping, ETL, or maintenance",
	},
	ShapeCLI:     {ID: ShapeCLI, Name: "CLI tool", Description: "A command-line application"},
	ShapeWorker:  {ID: ShapeWorker, Name: "Background worker", Description: "A long-running process that consumes or schedules work"},
	ShapeLibrary: {ID: ShapeLibrary, Name: "Go library", Description: "A reusable Go package"},
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
	ArchitecturePipeline: {
		ID:          ArchitecturePipeline,
		Name:        "Pipeline",
		Description: "Run ordered, context-aware stages and exit",
	},
}

var templatesByID = map[string]Template{
	TemplateGoMonolithLayered: newTemplate(
		TemplateGoMonolithLayered, "Go Monolith - Layered",
		"HTTP API starter with handler, service, and repository layers",
		ShapeMonolith, ArchitectureLayered, "",
	),
	TemplateGoMonolithClean: newTemplate(
		TemplateGoMonolithClean, "Go Monolith - Clean / Hexagonal",
		"HTTP API starter with use cases, ports, and adapters",
		ShapeMonolith, ArchitectureClean, "",
	),
	TemplateGoMonolithDDD: newTemplate(
		TemplateGoMonolithDDD, "Go Monolith - Domain-Driven Design",
		"HTTP API starter organized around bounded contexts",
		ShapeMonolith, ArchitectureDDD, "",
	),
	TemplateGoMicroservicesLayered: newTemplate(
		TemplateGoMicroservicesLayered, "Go Microservices - Layered",
		"go-micro service starter with handler, service, and repository layers",
		ShapeMicroservices, ArchitectureLayered, "go-micro",
	),
	TemplateGoMicroservicesClean: newTemplate(
		TemplateGoMicroservicesClean, "Go Microservices - Clean / Hexagonal",
		"go-micro service starter with use cases, ports, and adapters",
		ShapeMicroservices, ArchitectureClean, "go-micro",
	),
	TemplateGoMicroservicesDDD: newTemplate(
		TemplateGoMicroservicesDDD, "Go Microservices - Domain-Driven Design",
		"go-micro services organized around bounded contexts",
		ShapeMicroservices, ArchitectureDDD, "go-micro",
	),
	TemplateGoOneShotPipeline: newTemplate(
		TemplateGoOneShotPipeline, "Go One-shot Job - Pipeline",
		"Context-aware job pipeline for ETL, scraping, and maintenance work",
		ShapeOneShotJob, ArchitecturePipeline, "",
	),
	TemplateGoCLI: newTemplate(
		TemplateGoCLI, "Go CLI - Standard",
		"Command-line application with a clear command entry point",
		ShapeCLI, ArchitectureLayered, "",
	),
	TemplateGoWorker: newTemplate(
		TemplateGoWorker, "Go Background Worker - Standard",
		"Long-running worker with graceful shutdown and a replaceable work source",
		ShapeWorker, ArchitectureLayered, "",
	),
	TemplateGoLibrary: newTemplate(
		TemplateGoLibrary, "Go Library - Standard",
		"Reusable Go package with a small public API and example tests",
		ShapeLibrary, ArchitectureClean, "",
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
			Supported:   true,
		},
			{ID: StackNodeJS, Name: "Node.js", Description: "Planned; generator not available yet"},
			{ID: StackNestJS, Name: "NestJS", Description: "Planned; generator not available yet"},
			{ID: StackLaravel, Name: "Laravel", Description: "Planned; generator not available yet"},
			{ID: StackPHP, Name: "PHP", Description: "Planned; generator not available yet"},
		},
	},
	SettingServiceFramework: {
		ID:   SettingServiceFramework,
		Name: "Service framework",
		Options: []SettingOption{{
			ID:          "go-micro",
			Name:        "go-micro",
			Description: "Go framework for microservice discovery and communication",
			Supported:   true,
		}},
	},
}

func newTemplate(id, name, description, shapeID, architectureID, serviceFrameworkID string) Template {
	workloadID, topologyID := WorkloadForLegacyShape(shapeID)
	return Template{
		ID:                 id,
		Name:               name,
		Description:        description,
		StackID:            StackGo,
		AppShapeID:         shapeID,
		WorkloadID:         workloadID,
		TopologyID:         topologyID,
		ArchitectureID:     architectureID,
		ServiceFrameworkID: serviceFrameworkID,
	}
}

func WorkloadForLegacyShape(shapeID string) (string, string) {
	switch shapeID {
	case ShapeMonolith, ShapeMicroservices:
		return WorkloadHTTPAPI, shapeID
	case ShapeOneShotJob:
		return WorkloadOneShot, ""
	case ShapeCLI:
		return WorkloadCLI, ""
	case ShapeWorker:
		return WorkloadWorker, ""
	case ShapeLibrary:
		return WorkloadLibrary, ""
	default:
		return "", ""
	}
}

func Workloads() []Workload {
	return []Workload{
		{ID: WorkloadHTTPAPI, Name: "HTTP API / backend", Description: "A service that handles HTTP requests"},
		{ID: WorkloadCLI, Name: "CLI tool", Description: "A command-line application for people or scripts"},
		{ID: WorkloadOneShot, Name: "One-shot job or pipeline", Description: "Run once and exit: ETL, scraping, import/export, or maintenance"},
		{ID: WorkloadWorker, Name: "Background worker", Description: "Process work continuously or from a queue"},
		{ID: WorkloadLibrary, Name: "Go library", Description: "A reusable package for other Go programs"},
	}
}

func WorkloadByID(id string) (Workload, bool) {
	for _, workload := range Workloads() {
		if workload.ID == id {
			return workload, true
		}
	}
	return Workload{}, false
}

func AppShapes() []AppShape {
	return []AppShape{
		appShapesByID[ShapeMonolith],
		appShapesByID[ShapeMicroservices],
		appShapesByID[ShapeOneShotJob],
		appShapesByID[ShapeCLI],
		appShapesByID[ShapeWorker],
		appShapesByID[ShapeLibrary],
	}
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
		architecturesByID[ArchitecturePipeline],
	}
}

func ArchitecturesForShape(shapeID string) []Architecture {
	available := make(map[string]struct{})
	for _, template := range Templates() {
		if template.AppShapeID == shapeID {
			available[template.ArchitectureID] = struct{}{}
		}
	}
	var result []Architecture
	for _, architecture := range Architectures() {
		if _, ok := available[architecture.ID]; ok {
			result = append(result, architecture)
		}
	}
	return result
}

func ArchitectureByID(id string) (Architecture, bool) {
	architecture, ok := architecturesByID[id]
	return architecture, ok
}

func Templates() []Template {
	return []Template{
		templatesByID[TemplateGoMonolithLayered],
		templatesByID[TemplateGoMonolithClean],
		templatesByID[TemplateGoMonolithDDD],
		templatesByID[TemplateGoMicroservicesLayered],
		templatesByID[TemplateGoMicroservicesClean],
		templatesByID[TemplateGoMicroservicesDDD],
		templatesByID[TemplateGoOneShotPipeline],
		templatesByID[TemplateGoCLI],
		templatesByID[TemplateGoWorker],
		templatesByID[TemplateGoLibrary],
	}
}

func ArchitecturesForWorkload(workloadID, topologyID string) []Architecture {
	available := make(map[string]struct{})
	for _, template := range Templates() {
		if template.WorkloadID == workloadID && (topologyID == "" || template.TopologyID == topologyID) {
			available[template.ArchitectureID] = struct{}{}
		}
	}
	var result []Architecture
	for _, architecture := range Architectures() {
		if _, ok := available[architecture.ID]; ok {
			result = append(result, architecture)
		}
	}
	return result
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
