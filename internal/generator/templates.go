package generator

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"strings"
	texttemplate "text/template"
)

//go:embed templates
var templateFiles embed.FS

type part struct {
	source         string
	output         string
	patternPackage string
}

type templateData struct {
	Module             string
	Name               string
	Description        string
	TemplateID         string
	TemplateName       string
	Architecture       string
	Microservices      bool
	OneShot            bool
	GoVersion          string
	Requirements       []string
	Run                string
	PatternList        string
	Databases          []string
	AppImport          string
	ServiceName        string
	CatalogServiceName string
	LibraryPackage     string
	PatternPackage     string
}

func renderPart(item part, data templateData) (string, error) {
	source, err := templateFiles.ReadFile("templates/" + item.source)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", item.source, err)
	}
	parsed, err := texttemplate.New(item.source).Parse(string(source))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", item.source, err)
	}
	data.PatternPackage = item.patternPackage
	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", item.source, err)
	}
	return finalizeFile(item, rendered.Bytes())
}

func finalizeFile(item part, content []byte) (string, error) {
	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	if strings.HasSuffix(item.output, ".go") {
		formatted, err := format.Source(content)
		if err != nil {
			return "", fmt.Errorf("format template %s as %s: %w", item.source, item.output, err)
		}
		content = formatted
	}
	return strings.TrimRight(string(content), "\n") + "\n", nil
}
