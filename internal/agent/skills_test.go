package agent

import (
	"strings"
	"testing"
)

func TestSkillsFollowSelectedProfile(t *testing.T) {
	joined := strings.Join(SkillsForProfile(ProfileContext{
		Workload: "http-api", Patterns: []string{"rag", "chatbot"},
		Capabilities: []string{"database-mongodb", "delivery-deployment"},
	}), "\n")
	for _, want := range []string{"HTTP service", "RAG:", "Chatbot:", "MongoDB:", "Deployment:", "Security:", "Verification:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("profile skills missing %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "SQL database") {
		t.Error("Mongo profile received SQL instructions")
	}
}

func TestStandardBackendPatternLoadsServiceGuidance(t *testing.T) {
	joined := strings.Join(SkillsForProfile(ProfileContext{Workload: "cli", Patterns: []string{"standard-backend"}}), "\n")
	if !strings.Contains(joined, "HTTP service") {
		t.Fatalf("standard backend profile is missing service guidance: %s", joined)
	}
}
