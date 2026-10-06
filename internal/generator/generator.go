package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

// Generate creates a small, buildable Go starter in an already-created project directory.
// It refuses to overwrite any file so a profile can safely be generated only once.
func Generate(project projectstore.Project) error {
	if err := Validate(project); err != nil {
		return err
	}
	info, err := os.Stat(project.Path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project path must be an existing directory: %s", project.Path)
	}
	template, ok := catalog.TemplateByID(project.TemplateID)
	if !ok {
		return fmt.Errorf("unknown project template %q", project.TemplateID)
	}
	module := modulePath(project.Name)
	renderer, err := rendererFor(template)
	if err != nil {
		return err
	}
	files := renderer.Files(project, template, module)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(project.Path, filepath.FromSlash(name))
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing file %s", name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect %s: %w", name, err)
		}
	}
	for _, name := range names {
		contents := files[name]
		path := filepath.Join(project.Path, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", name, err)
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
		_, writeErr := file.WriteString(contents)
		closeErr := file.Close()
		if writeErr != nil {
			return fmt.Errorf("write %s: %w", name, writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", name, closeErr)
		}
	}
	return nil
}

func modulePath(name string) string {
	var clean strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '_' {
			clean.WriteRune(r)
		} else if clean.Len() > 0 {
			clean.WriteByte('-')
		}
	}
	value := strings.Trim(clean.String(), "-._")
	if value == "" {
		value = "app"
	}
	return "example.com/" + value
}

func goMod(module string, template catalog.Template) string {
	result := "module " + module + "\n\ngo 1.25.0\n"
	if template.AppShapeID == catalog.ShapeMicroservices {
		result += "\nrequire go-micro.dev/v6 v6.10.0\n"
	}
	return result
}

func mainGo(module string, template catalog.Template) string {
	if template.AppShapeID == catalog.ShapeMicroservices {
		appImport := module + "/internal/service"
		if template.ArchitectureID == catalog.ArchitectureClean {
			appImport = module + "/internal/application"
		} else if template.ArchitectureID == catalog.ArchitectureDDD {
			appImport = module + "/internal/greetings/application"
		}
		name := "projemble." + strings.ReplaceAll(strings.TrimPrefix(module, "example.com/"), "-", ".")
		return fmt.Sprintf(`package main

import (
	"context"
	"log"

	"go-micro.dev/v6"
	app %q
)

type HealthRequest struct{}
type HealthResponse struct { Status string }
type HealthService struct{}

func (*HealthService) Check(_ context.Context, _ *HealthRequest, response *HealthResponse) error {
	response.Status = app.Health()
	return nil
}

func main() {
	service := micro.NewService(%q)
	service.Init()
	if err := service.Handle(&HealthService{}); err != nil { log.Fatal(err) }
	if err := service.Run(); err != nil { log.Fatal(err) }
}
`, appImport, name)
	}
	var imports, setup, handler string
	switch template.ArchitectureID {
	case catalog.ArchitectureClean:
		imports = fmt.Sprintf("\"%s/internal/adapters/memory\"\n\t\"%s/internal/application\"\n\t\"%s/internal/transport/httpapi\"", module, module, module)
		setup, handler = "service := application.New(memory.New())", "httpapi.New(service)"
	case catalog.ArchitectureDDD:
		imports = fmt.Sprintf("\"%s/internal/greetings/application\"\n\t\"%s/internal/greetings/infrastructure\"\n\t\"%s/internal/greetings/interfaces/httpapi\"", module, module, module)
		setup, handler = "service := application.New(infrastructure.New())", "httpapi.New(service)"
	default:
		imports = fmt.Sprintf("\"%s/internal/handler\"", module)
		handler = "handler.New()"
	}
	return fmt.Sprintf(`package main

import (
	"log"
	"net/http"

%s
)

func main() {
	%s
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.Handle("/", %s)
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
`, imports, setup, handler)
}

func catalogServiceGo(module string) string {
	name := "projemble." + strings.ReplaceAll(strings.TrimPrefix(module, "example.com/"), "-", ".") + ".catalog"
	return fmt.Sprintf(`package main

import (
	"context"
	"log"

	"go-micro.dev/v6"
)

type ListRequest struct{}
type ListResponse struct { Items []string }
type CatalogService struct{}

func (*CatalogService) List(_ context.Context, _ *ListRequest, response *ListResponse) error {
	response.Items = []string{}
	return nil
}

func main() {
	service := micro.NewService(%q)
	service.Init()
	if err := service.Handle(&CatalogService{}); err != nil { log.Fatal(err) }
	if err := service.Run(); err != nil { log.Fatal(err) }
}
`, name)
}

func architectureFiles(module string, template catalog.Template) map[string]string {
	monolith := template.AppShapeID == catalog.ShapeMonolith
	files := map[string]string{}
	if template.ArchitectureID == catalog.ArchitectureClean {
		files = cleanFiles(module, monolith)
	} else if template.ArchitectureID == catalog.ArchitectureDDD {
		files = dddFiles(module, monolith)
	} else {
		files = layeredFiles(module, monolith)
	}
	return files
}

func layeredFiles(module string, monolith bool) map[string]string {
	files := map[string]string{
		"internal/service/service.go":   fmt.Sprintf("package service\n\nimport \"%s/internal/repository\"\n\nfunc Hello() string { return \"Project is running. \" + repository.Name() }\nfunc Health() string { return \"ok\" }\n", module),
		"internal/repository/memory.go": "package repository\n\nfunc Name() string { return \"layered starter\" }\n",
	}
	if monolith {
		files["internal/handler/http.go"] = fmt.Sprintf(`package handler

import (
	"fmt"
	"net/http"
	"%s/internal/service"
)

func New() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprintln(w, service.Hello()) })
	return mux
}
`, module)
	}
	return files
}

func cleanFiles(module string, monolith bool) map[string]string {
	files := map[string]string{
		"internal/domain/greeting.go":         "package domain\n\ntype Greeting string\n",
		"internal/ports/greeter.go":           fmt.Sprintf("package ports\n\nimport \"%s/internal/domain\"\n\ntype Greeter interface { Greet() domain.Greeting }\n", module),
		"internal/adapters/memory/greeter.go": fmt.Sprintf("package memory\n\nimport \"%s/internal/domain\"\n\ntype Greeter struct{}\nfunc New() Greeter { return Greeter{} }\nfunc (Greeter) Greet() domain.Greeting { return \"Project is running.\" }\n", module),
		"internal/application/service.go":     fmt.Sprintf("package application\n\nimport (\"%s/internal/domain\"; \"%s/internal/ports\")\n\ntype Service struct { greeter ports.Greeter }\nfunc New(greeter ports.Greeter) Service { return Service{greeter: greeter} }\nfunc (s Service) Greet() domain.Greeting { return s.greeter.Greet() }\nfunc Health() string { return \"ok\" }\n", module, module),
	}
	if monolith {
		files["internal/transport/httpapi/handler.go"] = fmt.Sprintf(`package httpapi

import (
	"fmt"
	"net/http"
	"%s/internal/application"
)

func New(service application.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprintln(w, service.Greet()) })
	return mux
}
`, module)
	}
	return files
}

func dddFiles(module string, monolith bool) map[string]string {
	files := map[string]string{
		"internal/greetings/domain/greeting.go":       "package domain\n\ntype Greeting struct { Message string }\nfunc NewGreeting(message string) Greeting { return Greeting{Message: message} }\n",
		"internal/greetings/infrastructure/memory.go": fmt.Sprintf("package infrastructure\n\nimport \"%s/internal/greetings/domain\"\n\ntype Repository struct{}\nfunc New() Repository { return Repository{} }\nfunc (Repository) Greeting() domain.Greeting { return domain.NewGreeting(\"Project is running.\") }\n", module),
		"internal/greetings/application/service.go":   fmt.Sprintf("package application\n\nimport \"%s/internal/greetings/domain\"\n\ntype Repository interface { Greeting() domain.Greeting }\ntype Service struct { repository Repository }\nfunc New(repository Repository) Service { return Service{repository: repository} }\nfunc (s Service) Greet() domain.Greeting { return s.repository.Greeting() }\nfunc Health() string { return \"ok\" }\n", module),
	}
	if monolith {
		files["internal/greetings/interfaces/httpapi/handler.go"] = fmt.Sprintf(`package httpapi

import (
	"fmt"
	"net/http"
	"%s/internal/greetings/application"
)

func New(service application.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprintln(w, service.Greet().Message) })
	return mux
}
`, module)
	}
	return files
}

func readme(project projectstore.Project, template catalog.Template, module string) string {
	var run string
	switch template.AppShapeID {
	case catalog.ShapeOneShotJob:
		run = "go run ./cmd/job"
	case catalog.ShapeMicroservices:
		run = "go mod tidy\n\n# Terminal 1\ngo run ./cmd/server\n\n# Terminal 2\ngo run ./cmd/catalog"
	default:
		run = "go run ./cmd/server"
	}
	result := fmt.Sprintf("# %s\n\n%s\n\nGenerated from Projemble template `%s` (%s).\n\n## Run\n\n```sh\n%s\n```\n\n## Verify\n\n```sh\ngo test ./...\ngo vet ./...\ngo build ./...\n```\n\nGo module: `%s`.\n", project.Name, project.Description, template.ID, template.Name, run, module)
	if template.AppShapeID == catalog.ShapeOneShotJob {
		result += "\n## Adapt the pipeline\n\nUse the ordered stages in `cmd/job/main.go` for your task. For a scraper, implement fetch and parse stages; for ETL, implement extract, transform, and load. Return an error from a failed stage to stop the run. The pipeline accepts a context and exits when the work is done.\n"
	}
	return result
}

// Validate checks the profile fields needed for generation without changing the filesystem.
func Validate(project projectstore.Project) error {
	if strings.TrimSpace(project.Path) == "" {
		return errors.New("project path is required")
	}
	if _, ok := catalog.TemplateByID(project.TemplateID); !ok {
		return fmt.Errorf("unknown project template %q", project.TemplateID)
	}
	return nil
}
