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
		"go.mod":     goMod(module, template),
		"README.md":  readme(project, template, module),
		".gitignore": "/bin/\n",
	}
}

func rendererFor(template catalog.Template) (templateRenderer, error) {
	switch template.AppShapeID {
	case catalog.ShapeMonolith, catalog.ShapeMicroservices:
		return serviceTemplateRenderer{}, nil
	case catalog.ShapeOneShotJob:
		return oneShotJobRenderer{}, nil
	default:
		return nil, fmt.Errorf("no renderer for application shape %q", template.AppShapeID)
	}
}
