package agent

import (
	"context"
	"strings"
	"unicode"
)

type AccessMode string

const (
	AccessReadOnly  AccessMode = "read-only"
	AccessFull      AccessMode = "full-access"
	AccessAskAlways AccessMode = "ask-always"
)

type PermissionRequest struct {
	Action string
	Target string
}

type PermissionApprover interface {
	RequestPermission(context.Context, PermissionRequest) (bool, error)
}

func (mode AccessMode) Valid() bool {
	switch mode {
	case AccessReadOnly, AccessFull, AccessAskAlways:
		return true
	default:
		return false
	}
}

func needsApproval(mode AccessMode, tool string) bool {
	return mode == AccessAskAlways || (mode == AccessFull && tool == "run_shell")
}

func (mode AccessMode) tools() []string {
	tools := []string{"list_files", "search_files", "read_file"}
	if mode == AccessFull || mode == AccessAskAlways {
		tools = append(tools, "write_file", "edit_file", "undo_last_edit", "run_command", "run_shell", "delegate_checks")
	}
	return tools
}

func cleanPermissionTarget(target string) string {
	target = strings.Map(func(value rune) rune {
		if unicode.IsControl(value) {
			return ' '
		}
		return value
	}, target)
	target = strings.Join(strings.Fields(target), " ")
	runes := []rune(target)
	if len(runes) > 240 {
		return string(runes[:240]) + "..."
	}
	return target
}
