package tui

import (
	"fmt"
	"image"
	"strings"
	"unicode/utf8"

	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"

	"projemble/internal/llm"
)

const (
	maxPromptRunes     = 32 << 10
	maxPendingPrompts  = 32
	agentSpinnerFrames = "|/-\\"
)

type agentWorkspace struct {
	header     *widgets.Paragraph
	transcript *transcriptView
	composer   *widgets.TextArea
	session    llm.Usage
	last       llm.Usage
	spinner    int
	details    bool
}

type workspaceAction struct {
	prompt string
	queued bool
	back   bool
	quit   bool
	scroll bool
	follow bool
}

func newAgentWorkspace() *agentWorkspace {
	header := widgets.NewParagraph()
	header.Border = true
	header.Title = "Projemble agent"
	transcript := newTranscriptView()
	transcript.Border = true
	transcript.Title = "Conversation and live activity"
	composer := widgets.NewTextArea()
	composer.Border = true
	composer.Title = "Message"
	composer.Text = ""
	composer.CursorStyle = focusedStyle()
	return &agentWorkspace{header: header, transcript: transcript, composer: composer}
}

func (workspace *agentWorkspace) Render(width, height int, rows []string, options generationOptions, path string, running, follow bool, queued int) {
	provider, _ := providerAtIndex(options.Provider)
	workspace.header.Title = fmt.Sprintf("Projemble agent  ·  %s  ·  %s", provider, options.Model)
	workspace.header.Text = workspace.statusText(path, running, queued)

	composerHeight := min(6, max(4, strings.Count(workspace.composer.Text, "\n")+3))
	if height < 12 {
		composerHeight = max(3, height/3)
	}
	headerHeight := min(6, max(height/3, 3))
	logBottom := max(headerHeight, height-composerHeight)
	workspace.header.SetRect(0, 0, width, headerHeight)
	workspace.transcript.SetRect(0, headerHeight, width, logBottom)
	workspace.transcript.content(rows, workspace.details, follow)
	workspace.composer.SetRect(0, logBottom, width, height)
	if running {
		workspace.composer.TitleBottom = fmt.Sprintf("%c Working · queued %d/%d · Enter queue · PgUp/PgDn review · Ctrl+C exit", agentSpinnerFrames[workspace.spinner%len(agentSpinnerFrames)], queued, maxPendingPrompts)
	} else {
		workspace.composer.TitleBottom = "Enter send · Ctrl+J newline · PgUp/PgDn review · Esc back · Ctrl+C exit"
	}
	workspace.composer.ShowCursor = true
	ui.Render(workspace.header, workspace.transcript, workspace.composer)
}

func (workspace *agentWorkspace) Tick() {
	workspace.spinner = (workspace.spinner + 1) % len(agentSpinnerFrames)
}

func (workspace *agentWorkspace) AddUsage(usage llm.Usage) {
	if !usage.Available {
		return
	}
	workspace.last = usage
	workspace.session.Available = true
	workspace.session.InputTokens += usage.InputTokens
	workspace.session.OutputTokens += usage.OutputTokens
	workspace.session.CachedInputTokens += usage.CachedInputTokens
	if usage.TotalTokens != 0 {
		workspace.session.TotalTokens += usage.TotalTokens
	} else {
		workspace.session.TotalTokens += usage.InputTokens + usage.OutputTokens
	}
}

func (workspace *agentWorkspace) Handle(event ui.Event, busy bool) workspaceAction {
	switch event.ID {
	case "<C-o>":
		workspace.details = !workspace.details
		return workspaceAction{}
	case "<C-c>":
		return workspaceAction{quit: true}
	case "<Escape>":
		if !busy {
			return workspaceAction{back: true}
		}
	case "<PageUp>":
		workspace.transcript.ScrollPageUp()
		return workspaceAction{scroll: true}
	case "<PageDown>":
		workspace.transcript.ScrollPageDown()
		return workspaceAction{scroll: true, follow: workspace.transcript.AtBottom()}
	}
	switch event.ID {
	case "<Enter>":
		prompt := strings.TrimSpace(workspace.composer.Text)
		if prompt == "" {
			return workspaceAction{}
		}
		workspace.composer.Text = ""
		workspace.composer.Cursor = image.Point{}
		return workspaceAction{prompt: prompt, queued: busy}
	case "<C-j>":
		workspace.composer.InsertNewline()
	case "<Backspace>", "<C-h>":
		workspace.composer.DeleteRune()
	case "<Left>":
		workspace.composer.MoveCursor(-1, 0)
	case "<Right>":
		workspace.composer.MoveCursor(1, 0)
	case "<Up>":
		workspace.composer.MoveCursor(0, -1)
	case "<Down>":
		workspace.composer.MoveCursor(0, 1)
	case "<Home>":
		workspace.composer.Cursor.X = 0
	case "<End>":
		lines := strings.Split(workspace.composer.Text, "\n")
		workspace.composer.Cursor.X = len([]rune(lines[workspace.composer.Cursor.Y]))
	case "<C-a>":
		workspace.composer.Cursor = image.Point{}
	case "<C-e>":
		lines := strings.Split(workspace.composer.Text, "\n")
		workspace.composer.Cursor.Y = len(lines) - 1
		workspace.composer.Cursor.X = len([]rune(lines[workspace.composer.Cursor.Y]))
	case "<Space>":
		workspace.insert(' ')
	default:
		if event.Type == ui.KeyboardEvent {
			for _, char := range event.ID {
				if char == '\n' || char == '\r' || char == '\t' {
					continue
				}
				workspace.insert(char)
			}
		}
	}
	return workspaceAction{}
}

func (workspace *agentWorkspace) insert(char rune) {
	if utf8.RuneCountInString(workspace.composer.Text) < maxPromptRunes {
		workspace.composer.InsertRune(char)
	}
}

func (workspace *agentWorkspace) statusText(path string, running bool, queued int) string {
	tokenLabel := "Usage:"
	tokens := "waiting for provider token counts"
	context := "token count and model limit not reported yet"
	state := "ready for instructions"
	if running {
		state = fmt.Sprintf("[%c](fg:%s,mod:bold) Agent is thinking · %d queued", agentSpinnerFrames[workspace.spinner%len(agentSpinnerFrames)], colorThinking, queued)
	}
	if workspace.session.Available {
		tokenLabel = "Session tokens:"
		tokens = fmt.Sprintf("%s input  ·  %s output  ·  %s total", formatTokens(workspace.session.InputTokens), formatTokens(workspace.session.OutputTokens), formatTokens(workspace.session.TotalTokens))
	}
	if workspace.last.Available {
		context = fmt.Sprintf("latest request used %s input tokens  ·  model window limit unavailable", formatTokens(workspace.last.InputTokens))
	}
	return fmt.Sprintf("%s %s\n%s %s\n%s %s\n%s %s", styleLabel("Status:"), state, styleLabel("Workspace:"), path, styleLabel(tokenLabel), tokens, styleLabel("Context:"), context)
}

func formatTokens(tokens int64) string {
	if tokens <= 0 {
		return "0"
	}
	value := fmt.Sprintf("%d", tokens)
	for i := len(value) - 3; i > 0; i -= 3 {
		value = value[:i] + "," + value[i:]
	}
	return value
}
