package tui

import (
	"fmt"
	"strings"

	"projemble/internal/agent"
)

const (
	commandCompact  = "/compact"
	commandNew      = "/new"
	commandSessions = "/sessions"
	commandResume   = "/resume"
)

type sessionCommand struct {
	name  string
	value string
}

func parseSessionCommand(prompt string) (sessionCommand, bool, error) {
	trimmed := strings.TrimSpace(prompt)
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return sessionCommand{}, false, nil
	}
	name := strings.ToLower(fields[0])
	value := strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))
	switch name {
	case commandCompact:
		return sessionCommand{name: "compact", value: value}, true, nil
	case commandNew:
		if value != "" && value != "carry" {
			return sessionCommand{}, true, fmt.Errorf("usage: /new [carry]")
		}
		return sessionCommand{name: "new", value: value}, true, nil
	case commandSessions:
		if value != "" {
			return sessionCommand{}, true, fmt.Errorf("usage: /sessions")
		}
		return sessionCommand{name: "sessions"}, true, nil
	case commandResume:
		if value == "" || strings.ContainsAny(value, " \t\r\n") {
			return sessionCommand{}, true, fmt.Errorf("usage: /resume <session-id>")
		}
		return sessionCommand{name: "resume", value: value}, true, nil
	default:
		return sessionCommand{}, false, nil
	}
}

func shortSessionID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func handleSessionCommand(prompt string, busy bool, session *agent.Agent, projectPath string, workspace **agentWorkspace, rows *[]string, follow *bool, startCompact func(string)) bool {
	command, ok, err := parseSessionCommand(prompt)
	if !ok {
		return false
	}
	if err != nil {
		*rows = appendActivity(*rows, err.Error())
		return true
	}
	if busy {
		*rows = appendActivity(*rows, "Finish the current response before using a session command.")
		return true
	}
	if session == nil {
		*rows = appendActivity(*rows, "Open an agent project before using session commands.")
		return true
	}
	switch command.name {
	case "compact":
		startCompact(command.value)
	case "new":
		previousID := session.SessionID()
		if err := session.StartNewSession(command.value == "carry"); err != nil {
			*rows = appendActivity(*rows, "Could not start a session: "+err.Error())
			return true
		}
		*workspace = newAgentWorkspace()
		*rows = appendActivity(nil, "New session started · previous session "+shortSessionID(previousID)+" is saved.")
		if command.value == "carry" {
			*rows = appendActivity(*rows, "Carried the previous session summary into this session.")
		}
	case "sessions":
		sessions, err := session.Sessions(projectPath)
		if err != nil {
			*rows = appendActivity(*rows, "Could not list sessions: "+err.Error())
			return true
		}
		(*workspace).sessions.open(sessions, session.SessionID())
	case "resume":
		if err := session.ResumeSession(projectPath, command.value); err != nil {
			*rows = appendActivity(*rows, "Could not resume session: "+err.Error())
			return true
		}
		*workspace = newAgentWorkspace()
		(*workspace).AddUsage(session.Usage())
		*rows = nil
		for _, row := range session.Transcript() {
			*rows = appendActivity(*rows, row)
		}
		*rows = appendActivity(*rows, "Resumed session "+shortSessionID(session.SessionID())+". Type your next instruction.")
	}
	*follow = true
	return true
}
