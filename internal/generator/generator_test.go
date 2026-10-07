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
			entrypoint, source := generatedPaths(template)
			for _, name := range []string{"README.md", "go.mod", entrypoint, source} {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
					t.Errorf("generated file %s missing: %v", name, err)
				}
			}
			mainPath := filepath.Join(root, filepath.FromSlash(entrypoint))
			mainSource, err := os.ReadFile(mainPath)
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
			if template.AppShapeID == catalog.ShapeOneShotJob && !strings.Contains(string(mainSource), "NewPipeline") {
				t.Error("one-shot template does not run a pipeline")
			}
			if err := verifyGenerated(root); err != nil {
				t.Fatalf("generated starter verification: %v", err)
			}
			if template.AppShapeID == catalog.ShapeOneShotJob {
				if err := runGo(root, "run", "./cmd/job"); err != nil {
					t.Fatalf("generated one-shot job does not run: %v", err)
				}
			}
			if err := Generate(project); err == nil {
				t.Error("second generation should refuse to overwrite existing files")
			}
		})
	}
}

func TestGenerateSupportedDatabaseAndPatternCombinations(t *testing.T) {
	for _, capability := range catalog.SupportedCapabilities("Primary database") {
		t.Run(capability.ID, func(t *testing.T) {
			root := t.TempDir()
			project := projectstore.Project{Name: "sample-app", Path: root, TemplateID: catalog.TemplateGoMonolithLayered, Capabilities: []string{capability.ID}}
			if err := Generate(project); err != nil {
				t.Fatal(err)
			}
			if err := runGo(root, "mod", "tidy"); err != nil {
				t.Fatalf("tidy generated module: %v", err)
			}
			if err := verifyGenerated(root); err != nil {
				t.Fatalf("verify generated module: %v", err)
			}
		})
	}
	t.Run("composed-patterns", func(t *testing.T) {
		root := t.TempDir()
		project := projectstore.Project{Name: "sample-app", Path: root, TemplateID: catalog.TemplateGoMonolithLayered, PatternIDs: []string{catalog.PatternBackend, catalog.PatternRAG, catalog.PatternAgent, catalog.PatternChatbot}, Capabilities: []string{catalog.CapabilitySQLite}}
		if err := Generate(project); err != nil {
			t.Fatal(err)
		}
		if err := runGo(root, "mod", "tidy"); err != nil {
			t.Fatalf("tidy generated module: %v", err)
		}
		if err := verifyGenerated(root); err != nil {
			t.Fatalf("verify generated patterns: %v", err)
		}
	})
}

func generatedPaths(template catalog.Template) (string, string) {
	switch template.WorkloadID {
	case catalog.WorkloadOneShot:
		return "cmd/job/main.go", "internal/job/pipeline.go"
	case catalog.WorkloadCLI:
		return "cmd/cli/main.go", "cmd/cli/main.go"
	case catalog.WorkloadWorker:
		return "cmd/worker/main.go", "cmd/worker/main.go"
	case catalog.WorkloadLibrary:
		return "greeting.go", "greeting_test.go"
	default:
		if template.ArchitectureID == catalog.ArchitectureClean {
			return "cmd/server/main.go", "internal/application/service.go"
		}
		if template.ArchitectureID == catalog.ArchitectureDDD {
			return "cmd/server/main.go", "internal/greetings/application/service.go"
		}
		return "cmd/server/main.go", "internal/service/service.go"
	}
}

func verifyGenerated(root string) error {
	for _, check := range [][]string{{"test", "./..."}, {"vet", "./..."}, {"build", "./..."}} {
		if err := runGo(root, check...); err != nil {
			return err
		}
	}
	return nil
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
