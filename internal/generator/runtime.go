package generator

import (
	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

func runtimeLogger(project projectstore.Project) string {
	if value := project.Choices[catalog.StepLogging]; value != "" {
		return value
	}
	if project.Preset == catalog.PresetRecommended {
		return "slog"
	}
	return "fmt"
}
func runtimeParts(project projectstore.Project) []part {
	parts := []part{
		{source: "runtime/config.go.tmpl", output: "internal/config/config.go"},
		{source: "runtime/logging.go.tmpl", output: "internal/platform/logging/logging.go"},
		{source: "runtime/respond.go.tmpl", output: "internal/platform/respond/respond.go"},
		{source: "runtime/health.go.tmpl", output: "internal/platform/health/health.go"},
		{source: "runtime/middleware.go.tmpl", output: "internal/platform/middleware/middleware.go"},
		{source: "runtime/httpserver.go.tmpl", output: "internal/platform/httpserver/server.go"},
	}
	if runtimeLogger(project) == "fmt" {
		parts = append(parts, part{source: "runtime/logging_fmt.go.tmpl", output: "internal/platform/logging/fmt.go"})
	}
	return parts
}
