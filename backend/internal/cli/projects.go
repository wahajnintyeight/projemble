package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

type settingFlags map[string]string

func (settings *settingFlags) String() string {
	return "key=value"
}

func (settings *settingFlags) Set(value string) error {
	key, settingValue, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return errors.New("setting must use key=value format")
	}
	if *settings == nil {
		*settings = make(map[string]string)
	}
	(*settings)[strings.TrimSpace(key)] = settingValue
	return nil
}

func RunProjects(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: projemble project <list|add>")
	}
	switch args[0] {
	case "list":
		return listProjects(stdout)
	case "add":
		return addProject(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown project command %q: use list or add", args[0])
	}
}

func listProjects(stdout io.Writer) error {
	config, err := projectstore.LoadDefault()
	if err != nil {
		return err
	}
	if len(config.Projects) == 0 {
		_, err := fmt.Fprintln(stdout, "No projects saved yet.")
		return err
	}
	for _, project := range config.Projects {
		fmt.Fprintf(stdout, "%s\n  %s\n  template: %s\n", project.Name, project.Path, project.TemplateID)
	}
	return nil
}

func addProject(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("project add", flag.ContinueOnError)
	flags.SetOutput(stderr)
	name := flags.String("name", "", "project name")
	description := flags.String("description", "", "short project description")
	path := flags.String("path", "", "project directory")
	templateID := flags.String("template", "", "template ID from the catalogue")
	settings := settingFlags{}
	flags.Var(&settings, "setting", "additional project setting as key=value (repeatable)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments after project add options")
	}
	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*path) == "" || strings.TrimSpace(*templateID) == "" {
		return errors.New("project add requires --name, --path, and --template")
	}
	template, ok := catalog.TemplateByID(*templateID)
	if !ok {
		return fmt.Errorf("unknown template %q", *templateID)
	}
	absolutePath, err := filepath.Abs(*path)
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}

	config, err := projectstore.LoadDefault()
	if err != nil {
		return err
	}
	project := projectstore.Project{
		Name:           strings.TrimSpace(*name),
		Description:    strings.TrimSpace(*description),
		Path:           absolutePath,
		StackID:        template.StackID,
		AppShapeID:     template.AppShapeID,
		ArchitectureID: template.ArchitectureID,
		TemplateID:     template.ID,
		Settings:       settings,
	}
	if err := config.Upsert(project); err != nil {
		return err
	}
	if err := projectstore.SaveDefault(config); err != nil {
		return err
	}
	configPath, err := projectstore.DefaultPath()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Saved %q to %s\n", project.Name, configPath)
	return err
}
