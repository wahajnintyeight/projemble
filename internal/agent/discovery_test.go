package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchAndRangedReadStayFocused(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("first\nSpecialMarker here\nthird\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "vendor", "dep.go"), []byte("SpecialMarker ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SpecialMarker secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	search, err := searchFiles(root, ".", "specialmarker")
	if err != nil || !strings.Contains(search, "src/main.go:2: SpecialMarker here") || strings.Contains(search, "vendor") || strings.Contains(search, ".env") || strings.Contains(search, "secret") {
		t.Fatalf("search result = %q, err=%v", search, err)
	}
	listed, err := listFiles(root, ".")
	if err != nil || strings.Contains(listed, ".env") || strings.Contains(listed, "vendor") {
		t.Fatalf("sensitive or vendored paths were listed: %q, err=%v", listed, err)
	}
	ranged, err := readFileRange(root, "src/main.go", 2, 2)
	if err != nil || !strings.Contains(ranged, "2 | SpecialMarker here") || strings.Contains(ranged, "first") {
		t.Fatalf("ranged read = %q, err=%v", ranged, err)
	}
	if _, err := decodeReadFileArguments(`{"path":"src/main.go","start_line":1,"end_line":1500}`); err == nil {
		t.Fatal("accepted an unbounded line range")
	}
}

func TestUndoRestoresOnlyUnchangedAgentEdits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	var history []editSnapshot
	if _, err := editFile(root, "note.txt", "before", "after", &history); err != nil {
		t.Fatal(err)
	}
	if result, err := undoLastEdit(root, &history); err != nil || result != "Undid edit to note.txt" {
		t.Fatalf("undo result = %q, err=%v", result, err)
	}
	if contents, _ := os.ReadFile(path); string(contents) != "before" {
		t.Fatalf("restored contents = %q", contents)
	}
	if _, err := createFile(root, "created.txt", "agent", &history); err != nil {
		t.Fatal(err)
	}
	if _, err := undoLastEdit(root, &history); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "created.txt")); !os.IsNotExist(err) {
		t.Fatalf("created file still exists: %v", err)
	}
	if _, err := editFile(root, "note.txt", "before", "agent", &history); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("user change"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := undoLastEdit(root, &history); err == nil {
		t.Fatal("undo overwrote a later user change")
	}
	if contents, _ := os.ReadFile(path); string(contents) != "user change" {
		t.Fatalf("user change was lost: %q", contents)
	}
}
