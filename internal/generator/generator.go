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

func Generate(project projectstore.Project) error {
	project = projectstore.NormalizeProject(project)
	files, err := renderProject(project)
	if err != nil {
		return err
	}
	if err := CheckToolchain(goLine(files["go.mod"])); err != nil {
		return err
	}
	info, err := os.Stat(project.Path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project path must be an existing directory: %s", project.Path)
	}
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
		path := filepath.Join(project.Path, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", name, err)
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
		_, writeErr := file.WriteString(files[name])
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

func renderProject(project projectstore.Project) (map[string]string, error) {
	project = projectstore.NormalizeProject(project)
	if err := Validate(project); err != nil {
		return nil, err
	}
	template, ok := catalog.TemplateByID(project.TemplateID)
	if !ok {
		return nil, fmt.Errorf("unknown project template %q", project.TemplateID)
	}
	parts, err := partsFor(project, template)
	if err != nil {
		return nil, err
	}
	data := newTemplateData(project, template, modulePath(project.Name))
	files := make(map[string]string, len(parts))
	for _, item := range parts {
		content, err := renderPart(item, data)
		if err != nil {
			return nil, err
		}
		files[item.output] = content
	}
	return files, nil
}

func newTemplateData(project projectstore.Project, template catalog.Template, module string) templateData {
	serviceName := "projemble." + strings.ReplaceAll(strings.TrimPrefix(module, "example.com/"), "-", ".")
	modules := projectModules(project, template.AppShapeID == catalog.ShapeMicroservices)
	data := templateData{
		Module:             module,
		Logger:             runtimeLogger(project),
		HTTPRuntime:        template.AppShapeID == catalog.ShapeMonolith,
		HasTests:           template.WorkloadID == catalog.WorkloadLibrary,
		Name:               project.Name,
		Description:        project.Description,
		TemplateID:         template.ID,
		TemplateName:       template.Name,
		Architecture:       template.ArchitectureID,
		Microservices:      template.AppShapeID == catalog.ShapeMicroservices,
		OneShot:            template.WorkloadID == catalog.WorkloadOneShot,
		GoVersion:          catalog.GoDirective(modules),
		Requirements:       requirementLines(modules),
		Run:                runCommand(template),
		AppImport:          module + applicationImportPath(template.ArchitectureID),
		ServiceName:        serviceName,
		CatalogServiceName: serviceName + ".catalog",
		LibraryPackage:     modulePackage(project.Name),
	}
	if len(project.PatternIDs) > 0 {
		data.PatternList = "`" + strings.Join(project.PatternIDs, "`, `") + "`"
	}
	for _, id := range project.Capabilities {
		if capability, ok := catalog.CapabilityByID(id); ok && databaseCapability(id) {
			data.Databases = append(data.Databases, capability.Name)
		}
	}
	return data
}

func applicationImportPath(architecture string) string {
	switch architecture {
	case catalog.ArchitectureClean:
		return "/internal/application"
	case catalog.ArchitectureDDD:
		return "/internal/greetings/application"
	default:
		return "/internal/service"
	}
}

func runCommand(template catalog.Template) string {
	switch template.WorkloadID {
	case catalog.WorkloadOneShot:
		return "go run ./cmd/job"
	case catalog.WorkloadCLI:
		return "go run ./cmd/cli"
	case catalog.WorkloadWorker:
		return "go run ./cmd/worker"
	case catalog.WorkloadLibrary:
		return "go test ./..."
	case catalog.WorkloadHTTPAPI:
		if template.TopologyID == catalog.TopologyMicroservices {
			return "go mod tidy\n\n# Terminal 1\ngo run ./cmd/server\n\n# Terminal 2\ngo run ./cmd/catalog"
		}
		return "go run ./cmd/server"
	}
	return ""
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

func Validate(project projectstore.Project) error {
	project = projectstore.NormalizeProject(project)
	if strings.TrimSpace(project.Path) == "" {
		return errors.New("project path is required")
	}
	template, ok := catalog.TemplateByID(project.TemplateID)
	if !ok {
		return fmt.Errorf("unknown project template %q", project.TemplateID)
	}
	if project.WorkloadID != template.WorkloadID || project.TopologyID != template.TopologyID {
		return fmt.Errorf("selected workload/topology do not match template %q", template.ID)
	}
	if template.AppShapeID == catalog.ShapeMonolith {
		if logger := runtimeLogger(project); logger != "fmt" && logger != "slog" {
			return fmt.Errorf("unsupported logging option %q", logger)
		}
	}
	seen := map[string]bool{}
	for _, id := range project.PatternIDs {
		if _, ok := catalog.PatternByID(id); !ok || seen[id] {
			return fmt.Errorf("unknown or duplicate application pattern %q", id)
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	primaryDatabase := false
	for _, id := range project.Capabilities {
		capability, ok := catalog.CapabilityByID(id)
		if !ok || !capability.Supported || seen[id] {
			return fmt.Errorf("capability %q is unknown, planned, or duplicated", id)
		}
		if capability.Category == "Primary database" {
			if primaryDatabase {
				return errors.New("choose at most one primary database")
			}
			primaryDatabase = true
		}
		seen[id] = true
	}
	return nil
}
