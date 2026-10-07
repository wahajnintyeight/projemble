package generator

import (
	"fmt"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

type cliRenderer struct{}

func (cliRenderer) Files(project projectstore.Project, template catalog.Template, module string) map[string]string {
	files := baseProjectFiles(project, template, module)
	files["cmd/cli/main.go"] = `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		fmt.Println("Usage: cli [--help]")
		return
	}
	fmt.Println("Project is ready.")
}
`
	return files
}

type workerRenderer struct{}

func (workerRenderer) Files(project projectstore.Project, template catalog.Template, module string) map[string]string {
	files := baseProjectFiles(project, template, module)
	files["cmd/worker/main.go"] = `package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Replace with queue consumption or scheduled work.
		}
	}
}
`
	return files
}

type libraryRenderer struct{}

func (libraryRenderer) Files(project projectstore.Project, template catalog.Template, module string) map[string]string {
	files := baseProjectFiles(project, template, module)
	files["greeting.go"] = fmt.Sprintf(`// Package %s is a reusable Go library.
package %s

// Greeting returns a short readiness message.
func Greeting() string { return "Project is ready." }
`, modulePackage(project.Name), modulePackage(project.Name))
	files["greeting_test.go"] = fmt.Sprintf(`package %s

import "testing"

func TestGreeting(t *testing.T) {
	if got := Greeting(); got == "" {
		t.Fatal("Greeting() returned an empty string")
	}
}
`, modulePackage(project.Name))
	return files
}

func modulePackage(name string) string {
	var result string
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			result += string(r)
		}
	}
	if result == "" || result[0] >= '0' && result[0] <= '9' {
		return "app"
	}
	return result
}
