package generator

import (
	"fmt"
	"os/exec"
	"strings"

	"projemble/internal/catalog"
)

func requirementLines(modules []catalog.ModuleInfo) []string {
	lines := make([]string, 0, len(modules))
	for _, module := range modules {
		lines = append(lines, module.Requirement())
	}
	return lines
}

func CheckToolchain(required string) error {
	output, err := exec.Command("go", "env", "GOVERSION", "GOTOOLCHAIN").Output()
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 || catalog.CompareGoVersions(fields[0], required) >= 0 || fields[1] != "local" {
		return nil
	}
	return fmt.Errorf("installed %s is older than Go %s used by generated projects and GOTOOLCHAIN=local blocks the automatic download; install a newer Go from https://go.dev/dl/", fields[0], required)
}

func goLine(goMod string) string {
	for _, line := range strings.Split(goMod, "\n") {
		if version, ok := strings.CutPrefix(line, "go "); ok {
			return strings.TrimSpace(version)
		}
	}
	return catalog.GoBaseline
}
