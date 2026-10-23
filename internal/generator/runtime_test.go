package generator

import (
	"embed"
	"os"
	"path/filepath"
	"projemble/internal/catalog"
	"projemble/internal/projectstore"
	"strings"
	"testing"
)

//go:embed testdata/behaviour
var behaviourFiles embed.FS

func TestHTTPRuntimeBehaviour(t *testing.T) {
	for _, templateID := range []string{catalog.TemplateGoMonolithLayered, catalog.TemplateGoMonolithClean, catalog.TemplateGoMonolithDDD} {
		for _, logger := range []string{"fmt", "slog"} {
			t.Run(templateID+"/"+logger, func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				project := projectstore.Project{Name: "runtime", Path: root, TemplateID: templateID, Choices: map[string]string{catalog.StepLogging: logger}}
				if err := Generate(project); err != nil {
					t.Fatal(err)
				}
				files, err := renderProject(project)
				if err != nil {
					t.Fatal(err)
				}
				for path := range files {
					if strings.HasSuffix(path, "_test.go") {
						t.Fatalf("tests leaked into default output: %s", path)
					}
				}
				template, _ := catalog.TemplateByID(templateID)
				data := newTemplateData(project, template, modulePath(project.Name))
				for _, part := range runtimeBehaviourParts() {
					raw, err := behaviourFiles.ReadFile("testdata/" + part.source)
					if err != nil {
						t.Fatal(err)
					}
					source, err := renderSource(part, data, raw)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(part.output)), []byte(source), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := assertQualityBar(root, false); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func runtimeBehaviourParts() []part {
	return []part{
		{source: "behaviour/config_test.go.tmpl", output: "internal/config/config_test.go"},
		{source: "behaviour/logging_test.go.tmpl", output: "internal/platform/logging/logging_test.go"},
		{source: "behaviour/respond_test.go.tmpl", output: "internal/platform/respond/respond_test.go"},
		{source: "behaviour/health_test.go.tmpl", output: "internal/platform/health/health_test.go"},
		{source: "behaviour/middleware_test.go.tmpl", output: "internal/platform/middleware/middleware_test.go"},
		{source: "behaviour/httpserver_test.go.tmpl", output: "internal/platform/httpserver/server_test.go"},
		{source: "behaviour/main_test.go.tmpl", output: "cmd/server/main_test.go"},
	}
}
