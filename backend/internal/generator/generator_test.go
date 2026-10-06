package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

func TestGenerateEveryCatalogTemplate(t *testing.T) {
	for _, template := range catalog.Templates() {
		t.Run(template.ID, func(t *testing.T) {
			root := t.TempDir()
			project := projectstore.Project{Name: "sample-app", Description: "A demo", Path: root, TemplateID: template.ID}
			if err := Generate(project); err != nil {
				t.Fatal(err)
			}
			applicationFile := "internal/service/service.go"
			if template.ArchitectureID == catalog.ArchitectureClean {
				applicationFile = "internal/application/service.go"
			} else if template.ArchitectureID == catalog.ArchitectureDDD {
				applicationFile = "internal/greetings/application/service.go"
			}
			for _, name := range []string{"README.md", "go.mod", "cmd/server/main.go", applicationFile} {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
					t.Errorf("generated file %s missing: %v", name, err)
				}
			}
			mainSource, err := os.ReadFile(filepath.Join(root, "cmd", "server", "main.go"))
			if err != nil {
				t.Fatal(err)
			}
			if template.AppShapeID == catalog.ShapeMicroservices && !strings.Contains(string(mainSource), "go-micro.dev/v6") {
				t.Error("microservice template does not initialize go-micro")
			}
			if template.AppShapeID == catalog.ShapeMicroservices {
				if _, err := os.Stat(filepath.Join(root, "cmd", "catalog", "main.go")); err != nil {
					t.Errorf("second service entrypoint missing: %v", err)
				}
				if err := runGo(root, "mod", "tidy"); err != nil {
					t.Fatalf("tidy generated module: %v", err)
				}
			}
			if err := runGo(root, "test", "./..."); err != nil {
				t.Fatalf("generated starter does not compile: %v", err)
			}
			if err := Generate(project); err == nil {
				t.Error("second generation should refuse to overwrite existing files")
			}
		})
	}
}

func runGo(dir string, args ...string) error {
	command := exec.Command("go", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, output)
	}
	return nil
}
