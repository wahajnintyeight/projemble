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
	config     Config
	provider   llm.Provider
	workspace  string
	messages   []llm.Message
	activities []string
	usage      llm.Usage
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
	return agent.Turn(ctx, workspace, task, output)
}

// Turn runs a user prompt and keeps the conversation for later turns on the
// same workspace. Activity and provider-reported usage are written as they
// arrive so a TUI can present a live session.
func (agent *Agent) Turn(ctx context.Context, workspace, task string, output io.Writer) error {
	if output == nil {
		output = io.Discard
	}
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
	if agent.workspace != "" && agent.workspace != root {
		return errors.New("agent session cannot change workspaces")
	}
	if len(agent.messages) == 0 {
		agent.workspace = root
		agent.messages = []llm.Message{{Role: "system", Content: SystemPrompt}}
	}
	agent.messages = append(agent.messages, llm.Message{Role: "user", Content: task})
	if err := agent.writeActivity(output, "You: "+task); err != nil {
		return err
	}
	if err := agent.checkpoint(); err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}
	request := llm.Request{Model: agent.config.Model, Tools: llmTools()}
	turn := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		turn++
		if err := agent.writeActivity(output, fmt.Sprintf("Waiting on %s model %s (turn %d)", agent.config.ProviderID, agent.config.Model, turn)); err != nil {
			return err
		}
		request.Messages = agent.messages
		response, err := agent.provider.Complete(ctx, request)
		if err != nil {
			_ = agent.writeActivity(output, "Provider request failed: "+err.Error())
			return err
		}
		if response.Usage.Available {
			agent.usage.Available = true
			agent.usage.InputTokens += response.Usage.InputTokens
			agent.usage.OutputTokens += response.Usage.OutputTokens
			agent.usage.TotalTokens += response.Usage.TotalTokens
		}
		if observer, ok := output.(interface{ ReportUsage(llm.Usage) }); ok {
			observer.ReportUsage(response.Usage)
		}
		message := response.Message
		if len(message.ToolCalls) == 0 {
			agent.messages = append(agent.messages, message)
			if message.Content != "" {
				if err := agent.writeActivity(output, "Agent summary: "+message.Content); err != nil {
					return err
				}
			}
			return agent.checkpoint()
		}
		if len(message.ToolCalls) > 8 {
			return errors.New("provider requested more than 8 tools in one response")
		}
		agent.messages = append(agent.messages, message)
		for _, call := range message.ToolCalls {
			if err := agent.writeActivity(output, "Action: "+toolAction(call.Name, call.Arguments)); err != nil {
				return err
			}
			result, err := runTool(ctx, root, call.Name, call.Arguments, agent.config.SecretEnvName)
			if err != nil {
				failure := "tool error: " + err.Error()
				if result != "" {
					result += "\n" + failure
				} else {
					result = failure
				}
			}
			if agent.config.APIKey != "" {
				result = strings.ReplaceAll(result, agent.config.APIKey, "[REDACTED]")
			}
			if err := agent.writeActivity(output, toolOutcome(call.Name, call.Arguments, result)); err != nil {
				return err
			}
			agent.messages = append(agent.messages, llm.Message{Role: "tool", ToolCallID: call.ID, ToolName: call.Name, Content: truncate(result, maxToolOutput)})
		}
		if err := agent.checkpoint(); err != nil {
			return fmt.Errorf("save conversation: %w", err)
		}
	}
}

func (agent *Agent) writeActivity(output io.Writer, text string) error {
	if agent.config.APIKey != "" {
		text = strings.ReplaceAll(text, agent.config.APIKey, "[REDACTED]")
	}
	agent.activities = append(agent.activities, text)
	if len(agent.activities) > 300 {
		agent.activities = agent.activities[len(agent.activities)-300:]
	}
	_, err := fmt.Fprintln(output, text)
	return err
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…[truncated]"
}
