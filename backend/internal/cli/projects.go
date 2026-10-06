package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"projemble/internal/agent"
	"projemble/internal/auth"
	"projemble/internal/catalog"
	"projemble/internal/llm"
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
		return errors.New("usage: projemble project <list|add|agent>")
	}
	switch args[0] {
	case "list":
		return listProjects(stdout)
	case "add":
		return addProject(args[1:], stdout, stderr)
	case "agent":
		return runAgent(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown project command %q: use list, add, or agent", args[0])
	}
}

func runAgent(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("project agent", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("path", "", "existing project directory")
	task := flags.String("task", "", "code task for the agent")
	providerName := flags.String("provider", "openai", "provider: openai, openai-web, claude, deepseek, mistral, qwen, openrouter, huggingface, gemini")
	baseURL := flags.String("base-url", "", "provider API base URL (uses provider default when omitted)")
	model := flags.String("model", "", "provider model ID")
	keyEnv := flags.String("api-key-env", "", "environment variable holding the provider API key")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*path) == "" || strings.TrimSpace(*task) == "" || strings.TrimSpace(*model) == "" {
		return errors.New("project agent requires --path, --task, and --model")
	}
	provider, envDefault, err := parseProvider(*providerName)
	if err != nil {
		return err
	}
	if *keyEnv == "" {
		*keyEnv = envDefault
	}
	if strings.TrimSpace(*keyEnv) == "" && provider != llm.OpenAIWeb {
		return errors.New("--api-key-env must name an environment variable")
	}
	key := ""
	if *keyEnv != "" {
		key = os.Getenv(*keyEnv)
	}
	config := agent.Config{ProviderID: provider, BaseURL: *baseURL, Model: *model, APIKey: key, SecretEnvName: *keyEnv}
	if provider == llm.OpenAIWeb {
		config.Credentials = &auth.ChatGPTTokenSource{}
	}
	client, err := agent.New(config)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	return client.Run(ctx, *path, *task, stdout)
}

func parseProvider(value string) (llm.ProviderID, string, error) {
	id := llm.ProviderID(strings.ToLower(strings.TrimSpace(value)))
	env := map[llm.ProviderID]string{llm.OpenAI: "OPENAI_API_KEY", llm.Claude: "ANTHROPIC_API_KEY", llm.DeepSeek: "DEEPSEEK_API_KEY", llm.Mistral: "MISTRAL_API_KEY", llm.Qwen: "DASHSCOPE_API_KEY", llm.OpenRouter: "OPENROUTER_API_KEY", llm.HuggingFace: "HF_TOKEN", llm.Gemini: "GEMINI_API_KEY"}
	if id == llm.OpenAIWeb {
		return id, "", nil
	}
	if _, ok := env[id]; !ok {
		return "", "", fmt.Errorf("unsupported provider %q", value)
	}
	return id, env[id], nil
}

func RunAuth(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] != "login" {
		return errors.New("usage: projemble auth login --provider openai-web")
	}
	flags := flag.NewFlagSet("auth login", flag.ContinueOnError)
	flags.SetOutput(stderr)
	provider := flags.String("provider", "openai-web", "provider authentication method")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *provider != "openai-web" {
		return errors.New("only openai-web supports browser sign-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return auth.Login(ctx, stdout)
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
