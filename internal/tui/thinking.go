package tui

import (
	"strings"

	"projemble/internal/llm"
)

var reasoningEfforts = []llm.ReasoningEffort{llm.ReasoningDefault, llm.ReasoningLow, llm.ReasoningMedium, llm.ReasoningHigh}

func nextReasoningEffort(current llm.ReasoningEffort) llm.ReasoningEffort {
	for i, effort := range reasoningEfforts {
		if effort == current {
			return reasoningEfforts[(i+1)%len(reasoningEfforts)]
		}
	}
	return llm.ReasoningDefault
}

func reasoningEffortLabel(effort llm.ReasoningEffort) string {
	if effort == llm.ReasoningDefault {
		return "Default (model decides)"
	}
	return string(effort)
}

func clearThinkingActivity(rows []string) []string {
	filtered := rows[:0]
	for _, row := range rows {
		if !strings.HasPrefix(row, "Thinking effort set to ") {
			filtered = append(filtered, row)
		}
	}
	return filtered
}
