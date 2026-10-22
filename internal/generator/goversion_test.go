package generator

import (
	"testing"

	"projemble/internal/catalog"
)

func TestGoLineReadsTheGeneratedDirective(t *testing.T) {
	files, err := renderProject(goldenCases()["template-"+catalog.TemplateGoMicroservicesLayered])
	if err != nil {
		t.Fatal(err)
	}
	if got := goLine(files["go.mod"]); got != "1.26.0" {
		t.Fatalf("goLine() = %q", got)
	}
	if got := goLine("module x\n"); got != catalog.GoBaseline {
		t.Fatalf("goLine() without a directive = %q", got)
	}
}

func TestProjectModulesIncludeFrameworkAndDatabase(t *testing.T) {
	modules := projectModules(goldenCases()["composed"], true)
	if len(modules) != 2 || modules[0].Path != "go-micro.dev/v6" || modules[1].Path != "modernc.org/sqlite" {
		t.Fatalf("modules = %v", modules)
	}
	if got := catalog.GoDirective(modules); got != "1.26.0" {
		t.Fatalf("go directive = %s", got)
	}
}

func TestCheckToolchainAcceptsCurrentInstall(t *testing.T) {
	if err := CheckToolchain("1.0.0"); err != nil {
		t.Fatal(err)
	}
}
