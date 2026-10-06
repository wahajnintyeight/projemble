package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateProjectParent(raw string) (string, error) {
	parent := strings.TrimSpace(raw)
	if parent == "" {
		return "", fmt.Errorf("project location is required")
	}
	if !filepath.IsAbs(parent) {
		return "", fmt.Errorf("enter an absolute project location for this operating system")
	}
	parent = filepath.Clean(parent)
	info, err := os.Stat(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("project location does not exist: %s", parent)
		}
		return "", fmt.Errorf("check project location %q: %w", parent, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project location must be a directory: %s", parent)
	}
	return parent, nil
}

func plannedProjectPath(parent, name string) (string, error) {
	if err := validateProjectName(name); err != nil {
		return "", err
	}
	parent, err := validateProjectParent(parent)
	if err != nil {
		return "", err
	}
	path := filepath.Join(parent, name)
	if _, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("project directory already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check project directory: %w", err)
	}
	return path, nil
}
