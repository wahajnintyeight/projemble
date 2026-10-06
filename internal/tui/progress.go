package tui

import (
	"context"
	"strings"
	"unicode"

	"projemble/internal/agent"
	"projemble/internal/llm"
)

const (
	maxActivityRows   = 300
	maxActivityRunes  = 500
	maxActivityOutput = 64 << 10
)

type generationUpdate struct {
	activity   string
	project    string
	configPath string
	session    *agent.Agent
	usage      llm.Usage
	hasUsage   bool
	err        error
	done       bool
}

type generationReporter struct {
	ctx     context.Context
	updates chan<- generationUpdate
	secret  string
	session *agent.Agent
}

func (reporter *generationReporter) SetAgentSession(session *agent.Agent) {
	reporter.session = session
}

func (reporter *generationReporter) AgentSession() *agent.Agent {
	return reporter.session
}

func (reporter *generationReporter) ReportUsage(usage llm.Usage) {
	if !usage.Available {
		return
	}
	select {
	case reporter.updates <- generationUpdate{usage: usage, hasUsage: true}:
	case <-reporter.ctx.Done():
	}
}

func (reporter generationReporter) Write(data []byte) (int, error) {
	written := len(data)
	clipped := len(data) > maxActivityOutput
	if len(data) > maxActivityOutput {
		data = data[:maxActivityOutput]
	}
	messageStart := strings.TrimLeft(string(data), " \t\r\n")
	if strings.HasPrefix(messageStart, "Agent summary:") || strings.HasPrefix(messageStart, "You:") {
		message := cleanAssistantMarkdown(string(data))
		if reporter.secret != "" {
			message = strings.ReplaceAll(message, reporter.secret, "[REDACTED]")
		}
		if clipped {
			message += "\n[response clipped for display]"
		}
		select {
		case reporter.updates <- generationUpdate{activity: message}:
			return written, nil
		case <-reporter.ctx.Done():
			return 0, reporter.ctx.Err()
		}
	}
	lines := strings.Split(string(data), "\n")
	if clipped {
		lines = append(lines, "[activity output clipped]")
	}
	for _, line := range lines {
		line = cleanActivity(line)
		if reporter.secret != "" {
			line = strings.ReplaceAll(line, reporter.secret, "[REDACTED]")
		}
		if line == "" {
			continue
		}
		select {
		case reporter.updates <- generationUpdate{activity: line}:
		case <-reporter.ctx.Done():
			return 0, reporter.ctx.Err()
		}
	}
	return written, nil
}

func appendActivity(rows []string, line string) []string {
	if strings.HasPrefix(strings.TrimSpace(line), "You:") {
		rows = appendTranscriptRow(rows, "")
		rows = appendTranscriptRow(rows, styledMarkdown("YOU", "fg:"+colorUser+",mod:bold"))
		for _, part := range strings.Split(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "You:")), "\n") {
			rows = appendTranscriptRow(rows, styledMarkdown(part, ""))
		}
		return appendTranscriptRow(rows, "")
	}
	if content, ok := assistantSummary(line); ok {
		return appendAssistantMarkdown(rows, content)
	}
	line = cleanActivity(line)
	if line == "" {
		return rows
	}
	if len([]rune(line)) > maxActivityRunes {
		line = string([]rune(line)[:maxActivityRunes]) + "..."
	}
	line = colorActivity(line)
	if len(rows) == maxActivityRows {
		copy(rows, rows[1:])
		rows[len(rows)-1] = line
		return rows
	}
	return append(rows, line)
}

func assistantSummary(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "Agent summary:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "Agent summary:")), true
}

func colorActivity(line string) string {
	marker, color := "", ""
	switch {
	case strings.HasPrefix(line, "Waiting on "):
		marker, color = "THINK", colorThinking
	case strings.HasPrefix(line, "You:") || strings.HasPrefix(line, "You (queued):"):
		marker, color = "YOU", colorUser
	case strings.Contains(line, "failed") || strings.Contains(line, "error:") || strings.HasPrefix(line, "Generation failed:"):
		marker, color = "ERROR", colorError
	case strings.HasPrefix(line, "Action: Reading ") || strings.HasPrefix(line, "Action: Listing ") || strings.HasPrefix(line, "Read ") || strings.HasPrefix(line, "Listed "):
		marker, color = "READ", colorRead
	case strings.HasPrefix(line, "Action: Writing "):
		marker, color = "EDIT", colorChanged
	case strings.HasPrefix(line, "Created ") || strings.HasPrefix(line, "Project created:"):
		marker, color = "+", colorAdded
	case strings.HasPrefix(line, "Updated ") || strings.HasPrefix(line, "Modified "):
		marker, color = "~", colorChanged
	case strings.HasPrefix(line, "Removed ") || strings.HasPrefix(line, "Deleted ") || strings.HasPrefix(line, "Action: Removing "):
		marker, color = "-", colorRemoved
	case strings.HasPrefix(line, "Action: Running ") || strings.HasPrefix(line, "Running "):
		marker, color = "CHECK", colorCheck
	case strings.Contains(line, " passed") || strings.HasSuffix(line, " passed") || strings.HasPrefix(line, "Ready for "):
		marker, color = "PASS", colorAdded
	case strings.HasPrefix(line, "Agent summary:") || strings.HasPrefix(line, "Starting next queued"):
		marker, color = "AI", colorThinking
	case strings.HasPrefix(line, "Cancellation requested") || strings.HasPrefix(line, "Cleared "):
		marker, color = "NOTE", colorChanged
	case strings.HasPrefix(line, "Profile saved:"):
		marker, color = "SAVE", colorRead
	}
	if marker == "" {
		return line
	}
	return "[" + marker + "](fg:" + color + ",mod:bold) " + line
}

func cleanActivity(line string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(line))
}
