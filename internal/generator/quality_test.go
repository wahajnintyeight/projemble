package generator

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func assertQualityBar(root string, includeTests bool) error {
	hasTests, err := staticQuality(root, includeTests)
	if err != nil || testing.Short() {
		return err
	}
	for _, args := range [][]string{{"mod", "tidy"}, {"mod", "tidy", "-diff"}, {"vet", "./..."}, {"build", "./..."}} {
		if err := runGo(root, args...); err != nil {
			return err
		}
	}
	if hasTests {
		args := []string{"test", "-count=1", "-timeout=60s"}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "go", "env", "CGO_ENABLED")
		command.Dir = root
		output, err := command.Output()
		if err != nil {
			return fmt.Errorf("check cgo: %w", err)
		}
		if strings.TrimSpace(string(output)) == "1" {
			args = append(args, "-race")
		}
		return runGo(root, append(args, "./...")...)
	}
	return nil
}

func staticQuality(root string, includeTests bool) (bool, error) {
	logic, tests := map[string]bool{}, map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
		if bytes.Count(bytes.TrimSuffix(content, []byte("\n")), []byte("\n"))+1 > 1000 {
			return fmt.Errorf("%s exceeds 1000 lines", path)
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		formatted, err := format.Source(content)
		if err != nil || !bytes.Equal(formatted, content) {
			return fmt.Errorf("%s is not gofmt-clean: %v", path, err)
		}
		tree, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
		if err != nil {
			return err
		}
		pkg := filepath.Dir(path)
		if strings.HasSuffix(path, "_test.go") {
			tests[pkg] = true
			return nil
		}
		imports := map[string]string{}
		for _, spec := range tree.Imports {
			name, _ := strconv.Unquote(spec.Path.Value)
			alias := filepath.Base(name)
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			imports[alias] = name
		}
		var violation error
		for _, declaration := range tree.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			logic[pkg] = true
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if selector.Sel.Name == "ListenAndServe" {
					violation = fmt.Errorf("%s uses ListenAndServe; use a configured server and managed listener", path)
				}
				receiver, ok := selector.X.(*ast.Ident)
				if ok && imports[receiver.Name] == "log" && strings.HasPrefix(selector.Sel.Name, "Fatal") && (tree.Name.Name != "main" || fn.Name.Name != "main") {
					violation = fmt.Errorf("%s uses log.Fatal outside main", path)
				}
				return true
			})
		}
		return violation
	})
	if err != nil {
		return false, err
	}
	if includeTests {
		for pkg := range logic {
			if !tests[pkg] {
				return false, fmt.Errorf("package %s has logic but no tests", pkg)
			}
		}
	}
	return len(tests) > 0, nil
}

func TestStaticQualityRules(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		tests        bool
		want         string
	}{
		{"format", "package p\nfunc F(){}", false, "gofmt"},
		{"server", "package p\n\nimport h \"net/http\"\n\nfunc F() { h.ListenAndServe(\":8080\", nil) }\n", false, "ListenAndServe"},
		{"fatal", "package p\n\nimport l \"log\"\n\nfunc F() { l.Fatal(\"x\") }\n", false, "log.Fatal"},
		{"coverage", "package p\n\nfunc F() {}\n", true, "no tests"},
		{"excluded", "package p\n\nfunc F() {}\n", false, ""},
		{"lines", "package p\n" + strings.Repeat("\n", 1000), false, "1000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "p.go"), []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := staticQuality(root, tc.tests)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v; want %q", err, tc.want)
			}
		})
	}
}
