package agent

import (
	"os"
	"projemble/internal/llm"
	"strings"
	"testing"
)

func TestSessionRestoresConversationAndRedactsCredentials(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	first := &Agent{config: Config{APIKey: "secret-key"}, workspace: root,
		messages:   []llm.Message{{Role: "user", Content: "Do not expose secret-key"}},
		activities: []string{"Created main.go"}, usage: llm.Usage{Available: true, TotalTokens: 42}}
	if err := first.checkpoint(); err != nil {
		t.Fatal(err)
	}
	// Overwriting an existing checkpoint must work on Windows too.
	if err := first.checkpoint(); err != nil {
		t.Fatal(err)
	}
	path, _ := sessionPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-key") {
		t.Fatal("credential leaked into session")
	}
	second := &Agent{}
	if err := second.Restore(root); err != nil {
		t.Fatal(err)
	}
	if len(second.messages) != 1 || second.Usage().TotalTokens != 42 || len(second.Transcript()) != 1 {
		t.Fatalf("state not restored: %+v", second)
	}
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := second.Restore(root); err == nil {
		t.Fatal("corrupt session accepted")
	}
}
