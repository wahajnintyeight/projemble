package generator

import (
	"fmt"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

var databaseTemplates = map[string]string{
	catalog.CapabilitySQLite:   "storage/sqlite.go.tmpl",
	catalog.CapabilityPostgres: "storage/postgres.go.tmpl",
	catalog.CapabilityMySQL:    "storage/mysql.go.tmpl",
	catalog.CapabilityMongoDB:  "storage/mongodb.go.tmpl",
}

var patternTemplates = map[string]string{
	catalog.PatternRAG:     "pattern/rag.go.tmpl",
	catalog.PatternAgent:   "pattern/agent.go.tmpl",
	catalog.PatternChatbot: "pattern/chatbot.go.tmpl",
}

const genericPatternTemplate = "pattern/generic.go.tmpl"

func partsFor(project projectstore.Project, template catalog.Template) ([]part, error) {
	parts := []part{
		{source: "base/go.mod.tmpl", output: "go.mod"},
		{source: "base/README.md.tmpl", output: "README.md"},
		{source: "base/gitignore.tmpl", output: ".gitignore"},
	}
	workload, err := workloadParts(template)
	if err != nil {
		return nil, err
	}
	parts = append(parts, workload...)
	if template.AppShapeID == catalog.ShapeMonolith {
		parts = append(parts, runtimeParts(project)...)
	}
	features, err := featureParts(project)
	if err != nil {
		return nil, err
	}
	return append(parts, features...), nil
}

func workloadParts(template catalog.Template) ([]part, error) {
	switch template.WorkloadID {
	case catalog.WorkloadHTTPAPI:
		return serviceParts(template), nil
	case catalog.WorkloadOneShot:
		return []part{
			{source: "job/main.go.tmpl", output: "cmd/job/main.go"},
			{source: "job/pipeline.go.tmpl", output: "internal/job/pipeline.go"},
		}, nil
	case catalog.WorkloadCLI:
		return []part{{source: "cli/main.go.tmpl", output: "cmd/cli/main.go"}}, nil
	case catalog.WorkloadWorker:
		return []part{{source: "worker/main.go.tmpl", output: "cmd/worker/main.go"}}, nil
	case catalog.WorkloadLibrary:
		return []part{
			{source: "library/greeting.go.tmpl", output: "greeting.go"},
			{source: "library/greeting_test.go.tmpl", output: "greeting_test.go"},
		}, nil
	default:
		return nil, fmt.Errorf("no renderer for workload %q", template.WorkloadID)
	}
}

func serviceParts(template catalog.Template) []part {
	var parts []part
	if template.AppShapeID == catalog.ShapeMicroservices {
		parts = append(parts,
			part{source: "service/micro_server.go.tmpl", output: "cmd/server/main.go"},
			part{source: "service/micro_catalog.go.tmpl", output: "cmd/catalog/main.go"},
		)
	} else {
		parts = append(parts, part{source: monolithServerTemplate(template.ArchitectureID), output: "cmd/server/main.go"})
	}
	return append(parts, architectureParts(template)...)
}

func monolithServerTemplate(architecture string) string {
	switch architecture {
	case catalog.ArchitectureClean:
		return "service/server_clean.go.tmpl"
	case catalog.ArchitectureDDD:
		return "service/server_ddd.go.tmpl"
	default:
		return "service/server_layered.go.tmpl"
	}
}

func architectureParts(template catalog.Template) []part {
	monolith := template.AppShapeID == catalog.ShapeMonolith
	var parts []part
	switch template.ArchitectureID {
	case catalog.ArchitectureClean:
		parts = []part{
			{source: "arch/clean/domain.go.tmpl", output: "internal/domain/greeting.go"},
			{source: "arch/clean/ports.go.tmpl", output: "internal/ports/greeter.go"},
			{source: "arch/clean/memory.go.tmpl", output: "internal/adapters/memory/greeter.go"},
			{source: "arch/clean/application.go.tmpl", output: "internal/application/service.go"},
		}
		if monolith {
			parts = append(parts, part{source: "arch/clean/handler.go.tmpl", output: "internal/transport/httpapi/handler.go"})
		}
	case catalog.ArchitectureDDD:
		parts = []part{
			{source: "arch/ddd/domain.go.tmpl", output: "internal/greetings/domain/greeting.go"},
			{source: "arch/ddd/infrastructure.go.tmpl", output: "internal/greetings/infrastructure/memory.go"},
			{source: "arch/ddd/application.go.tmpl", output: "internal/greetings/application/service.go"},
		}
		if monolith {
			parts = append(parts, part{source: "arch/ddd/handler.go.tmpl", output: "internal/greetings/interfaces/httpapi/handler.go"})
		}
	default:
		parts = []part{
			{source: "arch/layered/service.go.tmpl", output: "internal/service/service.go"},
			{source: "arch/layered/repository.go.tmpl", output: "internal/repository/memory.go"},
		}
		if monolith {
			parts = append(parts, part{source: "arch/layered/handler.go.tmpl", output: "internal/handler/http.go"})
		}
	}
	return parts
}

func featureParts(project projectstore.Project) ([]part, error) {
	var parts []part
	for _, id := range project.Capabilities {
		if !databaseCapability(id) {
			capability, ok := catalog.CapabilityByID(id)
			if !ok || !capability.Supported {
				return nil, fmt.Errorf("capability %q is planned and cannot be generated yet", id)
			}
			continue
		}
		parts = replacePart(parts, part{source: databaseTemplates[id], output: "internal/storage/database.go"})
	}
	for _, id := range project.PatternIDs {
		if _, ok := catalog.PatternByID(id); !ok {
			return nil, fmt.Errorf("unknown application pattern %q", id)
		}
		source, ok := patternTemplates[id]
		if !ok {
			source = genericPatternTemplate
		}
		parts = append(parts, part{source: source, output: "internal/pattern/" + id + "/pattern.go", patternPackage: patternPackage(id)})
	}
	return parts, nil
}

func replacePart(parts []part, replacement part) []part {
	for index := range parts {
		if parts[index].output == replacement.output {
			parts[index] = replacement
			return parts
		}
	}
	return append(parts, replacement)
}
