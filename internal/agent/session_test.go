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
		messages:     []llm.Message{{Role: "user", Content: "Do not expose secret-key"}},
		sessionTitle: "secret-key title",
		activities:   []string{"Created main.go"}, usage: llm.Usage{Available: true, TotalTokens: 42},
		summary: "Previous task summary", undoHistory: []editSnapshot{{Path: "main.go", Existed: true, Before: []byte("old"), BeforeMode: 0o644, AfterHash: strings.Repeat("a", 64), AfterMode: 0o644}}}
	if err := first.checkpoint(); err != nil {
		t.Fatal(err)
	}
	// Overwriting an existing checkpoint must work on Windows too.
	if err := first.checkpoint(); err != nil {
		t.Fatal(err)
	}
	path, _ := sessionFilePath(root, first.SessionID())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-key") {
		t.Fatal("credential leaked into session")
	}
	indexPath := sessionIndexPathForTest(t, root)
	indexData, err := os.ReadFile(indexPath)
	if err != nil || strings.Contains(string(indexData), "secret-key") {
		t.Fatalf("credential leaked into session index: %v %s", err, indexData)
	}
	second := &Agent{}
	if err := second.Restore(root); err != nil {
		t.Fatal(err)
	}
	if len(second.messages) != 2 || second.messages[0].Role != "system" || second.Usage().TotalTokens != 42 || len(second.Transcript()) != 1 || len(second.undoHistory) != 1 || second.summary != "Previous task summary" {
		t.Fatalf("state not restored: %+v", second)
	}
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := second.Restore(root); err == nil {
		t.Fatal("corrupt session accepted")
	}
}

func sessionIndexPathForTest(t *testing.T, root string) string {
	t.Helper()
	_, path, _, err := sessionPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewAndResumeSessionsPreservesPreviousConversation(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	current := &Agent{workspace: root, summary: "Earlier decisions", activities: []string{"Changed main.go"}, usage: llm.Usage{Available: true, TotalTokens: 23}}
	if err := current.checkpoint(); err != nil {
		t.Fatal(err)
	}
	previousID := current.SessionID()
	if err := current.StartNewSession(true); err != nil {
		t.Fatal(err)
	}
	newID := current.SessionID()
	if newID == previousID || current.summary != "Earlier decisions" || len(current.activities) != 0 || current.Usage().TotalTokens != 0 {
		t.Fatalf("new session state = id %q summary %q activities %v usage %+v", newID, current.summary, current.activities, current.Usage())
	}
	sessions, err := current.Sessions(root)
	if err != nil || len(sessions) != 2 {
		t.Fatalf("saved sessions = %d, err=%v", len(sessions), err)
	}
	if err := current.ResumeSession(root, previousID[:10]); err != nil {
		t.Fatal(err)
	}
	if current.SessionID() != previousID || current.summary != "Earlier decisions" || current.Usage().TotalTokens != 23 || len(current.Transcript()) != 1 {
		t.Fatalf("previous session was not restored: id=%q summary=%q usage=%+v transcript=%v", current.SessionID(), current.summary, current.Usage(), current.Transcript())
	}
	reopened := &Agent{}
	if err := reopened.Restore(root); err != nil || reopened.SessionID() != previousID {
		t.Fatalf("active session did not survive restart: id=%q err=%v", reopened.SessionID(), err)
	}
}
