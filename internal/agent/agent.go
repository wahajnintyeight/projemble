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
	"time"

	"projemble/internal/llm"
	"projemble/internal/llm/factory"
)

const (
	maxToolOutput           = 32 << 10
	maxProviderCallsPerTurn = 64
	maxToolCallsPerTurn     = 128
	maxToolCallsPerResponse = 8
	maxGoChecksPerTurn      = 6
	maxProviderCallDuration = 2 * time.Minute
)

type Config struct {
	ProviderID      llm.ProviderID
	BaseURL         string
	Model           string
	ReasoningEffort llm.ReasoningEffort
	APIKey          string
	SecretEnvName   string
	Credentials     llm.TokenSource
	Client          *http.Client
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

// SetReasoningEffort applies to the agent's next provider request.
func (agent *Agent) SetReasoningEffort(effort llm.ReasoningEffort) {
	agent.config.ReasoningEffort = effort
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
	if agent.workspace == "" {
		agent.workspace = root
	}
	agent.refreshSystemPrompt()
	agent.messages = append(agent.messages, llm.Message{Role: "user", Content: task})
	if err := agent.writeActivity(output, "You: "+task); err != nil {
		return err
	}
	if err := agent.checkpoint(); err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}
	request := llm.Request{Model: agent.config.Model, ReasoningEffort: agent.config.ReasoningEffort, Tools: llmTools()}
	providerCalls := 0
	toolCalls := 0
	goChecks := 0
	changedGoFiles := false
	goTestAttempted := false
	goTestPassed := false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if providerCalls >= maxProviderCallsPerTurn {
			return agent.stopWithGuardrail(output, fmt.Sprintf("agent task stopped after %d provider requests; send a follow-up instruction to continue", maxProviderCallsPerTurn))
		}
		providerCalls++
		requestLabel := fmt.Sprintf("Waiting on %s model %s", agent.config.ProviderID, agent.config.Model)
		if agent.config.ReasoningEffort != llm.ReasoningDefault {
			requestLabel += " · thinking " + string(agent.config.ReasoningEffort)
		}
		if err := agent.writeActivity(output, fmt.Sprintf("%s (request %d/%d)", requestLabel, providerCalls, maxProviderCallsPerTurn)); err != nil {
			return err
		}
		request.Messages = agent.messages
		requestContext, cancel := context.WithTimeout(ctx, maxProviderCallDuration)
		response, err := agent.provider.Complete(requestContext, request)
		cancel()
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
		if message.Role != "assistant" {
			return agent.stopWithGuardrail(output, fmt.Sprintf("provider response rejected: expected assistant message, got %q", message.Role))
		}
		if len(message.ToolCalls) == 0 {
			if strings.TrimSpace(message.Content) == "" {
				return agent.stopWithGuardrail(output, "provider returned an empty final response")
			}
			if changedGoFiles && hasGoModule(root) && !goTestPassed {
				status := "UNVERIFIED: no passing `go test ./...` was observed after the latest file changes."
				if goTestAttempted {
					status = "CHECKS FAILED: `go test ./...` did not pass after the latest file changes."
				}
				message.Content = status + "\n\n" + message.Content
				if err := agent.writeActivity(output, "Verification status: "+status); err != nil {
					return err
				}
			}
			agent.messages = append(agent.messages, message)
			if message.Content != "" {
				if err := agent.writeActivity(output, "Agent summary: "+message.Content); err != nil {
					return err
				}
			}
			return agent.checkpoint()
		}
		if len(message.ToolCalls) > maxToolCallsPerResponse {
			return agent.stopWithGuardrail(output, fmt.Sprintf("provider requested %d tools in one response; the safety limit is %d", len(message.ToolCalls), maxToolCallsPerResponse))
		}
		if toolCalls+len(message.ToolCalls) > maxToolCallsPerTurn {
			return agent.stopWithGuardrail(output, fmt.Sprintf("agent task reached its %d tool-action safety budget; send a follow-up instruction to continue", maxToolCallsPerTurn))
		}
		if err := validateToolCalls(root, message.ToolCalls, agent.config.ProviderID == llm.Gemini); err != nil {
			return agent.stopWithGuardrail(output, "provider tool request rejected by guardrail: "+err.Error())
		}
		if strings.TrimSpace(message.Content) != "" {
			if err := agent.writeActivity(output, "Agent: "+truncate(message.Content, maxToolOutput)); err != nil {
				return err
			}
		}
		agent.messages = append(agent.messages, message)
		for _, call := range message.ToolCalls {
			if err := agent.writeActivity(output, "Action: "+toolAction(call.Name, call.Arguments)); err != nil {
				return err
			}
			var result string
			var err error
			checkExecuted := true
			if call.Name == "run_go_check" && goChecks >= maxGoChecksPerTurn {
				result = fmt.Sprintf("guardrail: this task has reached the %d Go-check limit; continue with a follow-up instruction", maxGoChecksPerTurn)
				checkExecuted = false
			} else {
				if call.Name == "run_go_check" {
					goChecks++
				}
				result, err = runTool(ctx, root, call.Name, call.Arguments, agent.config.SecretEnvName)
			}
			if call.Name == "write_file" && err == nil && requiresGoCheck(validatedPath(call.Arguments)) {
				changedGoFiles = true
				goTestAttempted = false
				goTestPassed = false
			}
			if call.Name == "run_go_check" && validatedCheck(call.Arguments) == "test" && checkExecuted {
				goTestAttempted = true
				goTestPassed = err == nil
			}
			toolCalls++
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

func hasGoModule(root string) bool {
	info, err := os.Stat(filepath.Join(root, "go.mod"))
	return err == nil && info.Mode().IsRegular()
}

func validatedCheck(raw string) string {
	args, err := decodeToolArgs(raw, []string{"check"}, []string{"check"})
	if err != nil {
		return ""
	}
	return args["check"]
}

func validatedPath(raw string) string {
	args, err := decodeToolArgs(raw, []string{"path", "content"}, []string{"path", "content"})
	if err != nil {
		return ""
	}
	return args["path"]
}

func requiresGoCheck(name string) bool {
	name = strings.ToLower(filepath.ToSlash(filepath.Clean(name)))
	return strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum" || name == "go.work" ||
		strings.HasSuffix(name, "/go.mod") || strings.HasSuffix(name, "/go.sum") || strings.HasSuffix(name, "/go.work")
}

func (agent *Agent) stopWithGuardrail(output io.Writer, reason string) error {
	if err := agent.writeActivity(output, "Guardrail stop: "+reason); err != nil {
		return err
	}
	if err := agent.checkpoint(); err != nil {
		return fmt.Errorf("save stopped conversation: %w", err)
	}
	return errors.New(reason)
}

func (agent *Agent) refreshSystemPrompt() {
	messages := make([]llm.Message, 0, len(agent.messages)+1)
	messages = append(messages, llm.Message{Role: "system", Content: SystemPrompt})
	for _, message := range agent.messages {
		if message.Role != "system" {
			messages = append(messages, message)
		}
	}
	agent.messages = messages
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
