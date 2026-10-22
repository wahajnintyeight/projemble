package generator

import (
	"flag"
	"go/format"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

const goldenRoot = "testdata/golden"

func goldenCases() map[string]projectstore.Project {
	cases := map[string]projectstore.Project{}
	for _, template := range catalog.Templates() {
		cases["template-"+template.ID] = projectstore.Project{Name: "sample-app", Description: "A demo", Path: "unused", TemplateID: template.ID}
	}
	for _, database := range catalog.SupportedCapabilities("Primary database") {
		cases["db-"+database.ID] = projectstore.Project{Name: "sample-app", Path: "unused", TemplateID: catalog.TemplateGoMonolithLayered, Capabilities: []string{database.ID}}
	}
	for _, pattern := range catalog.Patterns() {
		cases["pattern-"+pattern.ID] = projectstore.Project{Name: "sample-app", Path: "unused", TemplateID: catalog.TemplateGoMonolithClean, PatternIDs: []string{pattern.ID}}
	}
	cases["composed"] = projectstore.Project{
		Name:         "sample-app",
		Path:         "unused",
		TemplateID:   catalog.TemplateGoMonolithLayered,
		PatternIDs:   []string{catalog.PatternBackend, catalog.PatternRAG, catalog.PatternAgent, catalog.PatternChatbot},
		Capabilities: []string{catalog.CapabilitySQLite},
	}
	return cases
}

func sortedCaseNames(cases map[string]projectstore.Project) []string {
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestGoldenOutput(t *testing.T) {
	cases := goldenCases()
	for _, name := range sortedCaseNames(cases) {
		t.Run(name, func(t *testing.T) {
			files, err := renderProject(cases[name])
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(goldenRoot, name)
			if *updateGolden {
				writeGolden(t, dir, files)
			}
			assertGolden(t, dir, files)
		})
	}
}

func writeGolden(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func assertGolden(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	expected := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		expected[filepath.ToSlash(relative)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("read golden directory (run with -update to create it): %v", err)
	}
	for name, content := range files {
		want, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("golden file %s missing: %v", name, err)
			continue
		}
		if strings.ReplaceAll(string(want), "\r\n", "\n") != content {
			t.Errorf("%s differs from golden file; review the change and run with -update", name)
		}
		delete(expected, name)
	}
	for name := range expected {
		t.Errorf("golden file %s is no longer generated", name)
	}
}

func TestGeneratedGoIsFormattedAndCommentFree(t *testing.T) {
	cases := goldenCases()
	for _, name := range sortedCaseNames(cases) {
		files, err := renderProject(cases[name])
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for output, content := range files {
			if !strings.HasSuffix(output, ".go") {
				continue
			}
			formatted, err := format.Source([]byte(content))
			if err != nil || string(formatted) != content {
				t.Errorf("%s/%s is not gofmt-clean: %v", name, output, err)
			}
			if comment := firstComment(content); comment != "" {
				t.Errorf("%s/%s contains a comment: %s", name, output, comment)
			}
		}
	}
}

func firstComment(source string) string {
	fileSet := token.NewFileSet()
	file := fileSet.AddFile("generated.go", fileSet.Base(), len(source))
	var lexer scanner.Scanner
	lexer.Init(file, []byte(source), nil, scanner.ScanComments)
	for {
		_, tok, literal := lexer.Scan()
		if tok == token.EOF {
			return ""
		}
		if tok == token.COMMENT {
			return literal
		}
	}
}

func TestEveryTemplateFileIsUsedAndEveryPartExists(t *testing.T) {
	used := map[string]bool{}
	for _, project := range goldenCases() {
		project = projectstore.NormalizeProject(project)
		template, _ := catalog.TemplateByID(project.TemplateID)
		parts, err := partsFor(project, template)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range parts {
			if _, err := templateFiles.ReadFile("templates/" + item.source); err != nil {
				t.Errorf("template %s is referenced but missing: %v", item.source, err)
			}
			used["templates/"+item.source] = true
		}
	}
	err := fs.WalkDir(templateFiles, "templates", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		if !used[path] {
			t.Errorf("template file %s is not used by any generated project", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeFileRejectsInvalidGoAndNormalizesText(t *testing.T) {
	_, err := finalizeFile(part{source: "broken.go.tmpl", output: "broken.go"}, []byte("package"))
	if err == nil || !strings.Contains(err.Error(), "broken.go.tmpl") {
		t.Fatalf("invalid Go error = %v, want it to name the template", err)
	}
	got, err := finalizeFile(part{source: "notes.tmpl", output: "NOTES.md"}, []byte("one\r\ntwo\r\n\r\n\r\n"))
	if err != nil || got != "one\ntwo\n" {
		t.Fatalf("finalizeFile() = %q, %v; want LF endings and one trailing newline", got, err)
	}
}
