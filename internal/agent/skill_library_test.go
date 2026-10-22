package agent

import (
	"strings"
	"testing"

	"projemble/internal/llm"
)

func TestPackagedEfficientCodeSkillLoads(t *testing.T) {
	skill, err := readSkill("efficient-code")
	if err != nil || !strings.Contains(skill, "# Efficient Code") {
		t.Fatalf("efficient-code SKILL.md not loaded: %v", err)
	}
	reference, err := readSkill("efficient-code/references/database.md")
	if err != nil || !strings.Contains(reference, "Database performance") {
		t.Fatalf("database reference not loaded: %v", err)
	}
	if !strings.Contains(packagedSkillIndex(), "efficient-code: Write, refactor") {
		t.Fatalf("skill index is missing efficient-code: %s", packagedSkillIndex())
	}
}

func TestReadSkillRejectsEscapesAndUnknownNames(t *testing.T) {
	for _, name := range []string{"", "../skills.go", "/etc/passwd", "efficient-code/../../tools.go", "efficient-code/SKILL.txt", "missing"} {
		if _, err := readSkill(name); err == nil {
			t.Errorf("readSkill(%q) succeeded", name)
		}
	}
}

func TestReadSkillIsAvailableInReadOnlyMode(t *testing.T) {
	calls := []llm.ToolCall{{ID: "1", Name: "read_skill", Arguments: `{"name":"efficient-code"}`}}
	if err := validateToolCallsForAccess(".", calls, false, AccessReadOnly); err != nil {
		t.Fatalf("read_skill blocked in read-only mode: %v", err)
	}
	found := false
	for _, tool := range llmToolsForAccess(AccessReadOnly) {
		found = found || tool.Name == "read_skill"
	}
	if !found {
		t.Fatal("read_skill is not advertised in read-only mode")
	}
}

func TestBackendProfilesReceiveEfficiencyGuidance(t *testing.T) {
	joined := strings.Join(SkillsForProfile(ProfileContext{Workload: "http-api"}), "\n")
	if !strings.Contains(joined, "Efficiency:") {
		t.Fatalf("http-api profile is missing efficiency guidance: %s", joined)
	}
	if strings.Contains(strings.Join(SkillsForProfile(ProfileContext{Workload: "library"}), "\n"), "Efficiency:") {
		t.Fatal("library profile received backend efficiency guidance")
	}
}
