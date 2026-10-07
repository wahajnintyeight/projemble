package generator

import (
	"fmt"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

type templateRenderer interface {
	Files(project projectstore.Project, template catalog.Template, module string) map[string]string
}

type serviceTemplateRenderer struct{}

func (serviceTemplateRenderer) Files(project projectstore.Project, template catalog.Template, module string) map[string]string {
	files := baseProjectFiles(project, template, module)
	files["cmd/server/main.go"] = mainGo(module, template)
	for name, content := range architectureFiles(module, template) {
		files[name] = content
	}
	if template.AppShapeID == catalog.ShapeMicroservices {
		files["cmd/catalog/main.go"] = catalogServiceGo(module)
	}
	return files
}

func baseProjectFiles(project projectstore.Project, template catalog.Template, module string) map[string]string {
	return map[string]string{
		"go.mod":     goModForProject(project, module, template),
		"README.md":  readme(project, template, module),
		".gitignore": "/bin/\n*.db\n.env\n",
	}
}

func rendererFor(template catalog.Template) (templateRenderer, error) {
	switch template.WorkloadID {
	case catalog.WorkloadHTTPAPI:
		return serviceTemplateRenderer{}, nil
	case catalog.WorkloadOneShot:
		return oneShotJobRenderer{}, nil
	case catalog.WorkloadCLI:
		return cliRenderer{}, nil
	case catalog.WorkloadWorker:
		return workerRenderer{}, nil
	case catalog.WorkloadLibrary:
		return libraryRenderer{}, nil
	default:
		return nil, fmt.Errorf("no renderer for workload %q", template.WorkloadID)
	}
}
