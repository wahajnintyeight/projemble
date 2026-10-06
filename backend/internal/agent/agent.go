package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"projemble/internal/llm"
	"projemble/internal/llm/factory"
)

const (
	maxToolOutput = 32 << 10
	maxRounds     = 8
)

type Config struct {
	ProviderID    llm.ProviderID
	BaseURL       string
	Model         string
	APIKey        string
	SecretEnvName string
	Credentials   llm.TokenSource
	Client        *http.Client
}

type Agent struct {
	config   Config
	provider llm.Provider
}

func New(config Config) (*Agent, error) {
	if strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("agent model is required")
	}
	provider, err := factory.New(llm.Config{
		Provider:    config.ProviderID,
		Model:       config.Model,
		APIKey:      config.APIKey,
		BaseURL:     config.BaseURL,
		Client:      config.Client,
		Credentials: config.Credentials,
	})
	if err != nil {
		return nil, err
	}
	return &Agent{config: config, provider: provider}, nil
}

// Run asks the selected provider to complete a code task using workspace tools.
func (agent *Agent) Run(ctx context.Context, workspace, task string, output io.Writer) error {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("workspace is not an existing directory: %s", workspace)
	}
	if strings.TrimSpace(task) == "" {
		return errors.New("agent task is required")
	}
	if len(task) > 32<<10 {
		return errors.New("agent task exceeds 32 KiB")
	}
	messages := []llm.Message{
		{Role: "system", Content: SystemPrompt},
		{Role: "user", Content: task},
	}
	request := llm.Request{Model: agent.config.Model, Tools: llmTools()}
	for round := 0; round < maxRounds; round++ {
		request.Messages = messages
		response, err := agent.provider.Complete(ctx, request)
		if err != nil {
			return err
		}
		message := response.Message
		if message.Content != "" {
			if _, err := fmt.Fprintln(output, message.Content); err != nil {
				return err
			}
		}
		if len(message.ToolCalls) == 0 {
			return nil
		}
		if len(message.ToolCalls) > 8 {
			return errors.New("provider requested more than 8 tools in one response")
		}
		messages = append(messages, message)
		for _, call := range message.ToolCalls {
			result, err := runTool(ctx, root, call.Name, call.Arguments, agent.config.SecretEnvName)
			if err != nil {
				result = "tool error: " + err.Error()
			}
			if agent.config.APIKey != "" {
				result = strings.ReplaceAll(result, agent.config.APIKey, "[REDACTED]")
			}
			messages = append(messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: truncate(result, maxToolOutput)})
		}
	}
	return fmt.Errorf("agent reached the %d tool-round limit", maxRounds)
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…[truncated]"
}
