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
	permission *agent.PermissionRequest
	decision   chan bool
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

func (reporter *generationReporter) RequestPermission(ctx context.Context, request agent.PermissionRequest) (bool, error) {
	decision := make(chan bool, 1)
	requestCopy := request
	select {
	case reporter.updates <- generationUpdate{permission: &requestCopy, decision: decision}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	select {
	case approved := <-decision:
		return approved, nil
	case <-ctx.Done():
		return false, ctx.Err()
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
	label, detail, color := "", "", ""
	switch {
	case strings.HasPrefix(line, "Waiting on "):
		label, color = "... think", colorThinking
		detail = strings.TrimPrefix(line, "Waiting on ")
		detail = strings.Replace(detail, " model ", " · ", 1)
		detail = strings.Replace(detail, " (request ", " · request ", 1)
		detail = strings.TrimSuffix(detail, ")")
	case strings.HasPrefix(line, "STDOUT "):
		label, color = "· out", colorRead
		detail = strings.TrimPrefix(line, "STDOUT ")
	case strings.HasPrefix(line, "STDERR "):
		label, color = "· err", colorError
		detail = strings.TrimPrefix(line, "STDERR ")
	case strings.HasPrefix(line, "You:") || strings.HasPrefix(line, "You (queued):"):
		label, color = "YOU", colorUser
		detail = strings.TrimPrefix(strings.TrimPrefix(line, "You (queued):"), "You:")
	case strings.HasPrefix(line, "Command failed:") || strings.HasPrefix(line, "Generation failed:") ||
		strings.HasPrefix(line, "Provider request failed:") || strings.Contains(strings.ToLower(line), "tool error:") ||
		strings.Contains(strings.ToLower(line), " failed") || strings.Contains(strings.ToLower(line), "error:"):
		label, color = "! failed", colorError
		detail = failureDetail(line)
	case strings.HasPrefix(line, "Action: Reading ") || strings.HasPrefix(line, "Reading "):
		label, color = "> read", colorRead
		detail = strings.TrimPrefix(strings.TrimPrefix(line, "Action: "), "Reading ")
	case strings.HasPrefix(line, "Action: Listing ") || strings.HasPrefix(line, "Listing "):
		label, color = "> list", colorRead
		detail = strings.TrimPrefix(strings.TrimPrefix(line, "Action: "), "Listing ")
	case strings.HasPrefix(line, "Action: Writing ") || strings.HasPrefix(line, "Writing "):
		label, color = "> write", colorChanged
		detail = strings.TrimPrefix(strings.TrimPrefix(line, "Action: "), "Writing ")
	case strings.HasPrefix(line, "Action: Removing ") || strings.HasPrefix(line, "Removing "):
		label, color = "> remove", colorRemoved
		detail = strings.TrimPrefix(strings.TrimPrefix(line, "Action: "), "Removing ")
	case strings.HasPrefix(line, "Action: Running ") || strings.HasPrefix(line, "Running "):
		label, color = "> run", colorCheck
		detail = strings.TrimPrefix(strings.TrimPrefix(line, "Action: "), "Running ")
	case strings.HasPrefix(line, "Action: Calling "):
		label, color = "> call", colorRead
		detail = strings.TrimPrefix(line, "Action: Calling ")
	case strings.HasPrefix(line, "Read "):
		label, color = "read", colorRead
		detail = strings.TrimPrefix(line, "Read ")
	case strings.HasPrefix(line, "Listed "):
		label, color = "read", colorRead
		detail = strings.TrimPrefix(line, "Listed ")
	case strings.HasPrefix(line, "Created ") || strings.HasPrefix(line, "Project created:"):
		label, color = "+ created", colorAdded
		detail = strings.TrimPrefix(strings.TrimPrefix(line, "Project created:"), "Created ")
	case strings.HasPrefix(line, "Updated ") || strings.HasPrefix(line, "Modified "):
		label, color = "~ updated", colorChanged
		detail = strings.TrimPrefix(strings.TrimPrefix(line, "Updated "), "Modified ")
	case strings.HasPrefix(line, "Removed ") || strings.HasPrefix(line, "Deleted "):
		label, color = "- removed", colorRemoved
		detail = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "Removed "), "Deleted "), "Action: Removing ")
	case strings.HasPrefix(line, "Command completed:"):
		label, color = "ok", colorAdded
		detail = strings.TrimPrefix(line, "Command completed:")
	case strings.Contains(line, " passed") || strings.HasSuffix(line, " passed"):
		label, color = "ok", colorAdded
		detail = strings.TrimSuffix(line, " passed")
	case strings.HasPrefix(line, "Ready for "):
		label, color = "ok", colorAdded
		detail = "ready for your next instruction"
	case strings.HasPrefix(line, "Starting next queued"):
		label, color = "· note", colorThinking
		detail = "starting next queued instruction"
	case strings.HasPrefix(line, "Cancellation requested") || strings.HasPrefix(line, "Cleared "):
		label, color = "· note", colorChanged
		detail = line
	case strings.HasPrefix(line, "Command skipped:"):
		label, color = "· skipped", colorChanged
		detail = strings.TrimPrefix(line, "Command skipped:")
	case strings.HasPrefix(line, "Profile saved:"):
		label, color = "+ saved", colorAdded
		detail = strings.TrimPrefix(line, "Profile saved:")
	}
	if label == "" {
		return line
	}
	row := styledMarkdown(label, "fg:"+color+",mod:bold")
	if detail != "" {
		row += styledMarkdown(" "+strings.TrimSpace(detail), "")
	}
	return row
}

func failureDetail(line string) string {
	for _, prefix := range []string{"Command failed:", "Generation failed:", "Provider request failed:", "tool error:"} {
		if strings.HasPrefix(strings.ToLower(line), strings.ToLower(prefix)) {
			line = strings.TrimSpace(line[len(prefix):])
			break
		}
	}
	line = strings.Replace(line, " failed: ", " · ", 1)
	line = strings.TrimSuffix(line, " failed")
	return line
}

func cleanActivity(line string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(line))
}
