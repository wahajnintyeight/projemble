package tui

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanMentionTreeIndexesNestedPathsAndSkipsGeneratedFolders(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"src/server/handlers.go",
		"docs/server-guide.md",
		"node_modules/package/index.js",
		".git/config",
		"dist/app.js",
	} {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("sample"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	paths, truncated, err := scanMentionTree(context.Background(), root, 100)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Fatal("small project was unexpectedly truncated")
	}
	for _, want := range []string{"src/server/handlers.go", "docs/server-guide.md"} {
		if !containsMentionPath(paths, want) {
			t.Errorf("nested path %q missing from index: %v", want, paths)
		}
	}
	for _, excluded := range []string{"node_modules", "node_modules/package/index.js", ".git", ".git/config", "dist", "dist/app.js"} {
		if containsMentionPath(paths, excluded) {
			t.Errorf("generated path %q should not be indexed", excluded)
		}
	}
	ranked := rankMentionPaths(paths, "handlers", 10)
	if len(ranked) != 1 || ranked[0] != "src/server/handlers.go" {
		t.Fatalf("unexpected ranked matches: %v", ranked)
	}
}

func TestFileMentionUsesCompletedProjectIndexForGlobalSearch(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "internal", "service", "server.go")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package service"), 0600); err != nil {
		t.Fatal(err)
	}
	composer := newMessageComposer()
	composer.Text = "@server"
	composer.Cursor = image.Pt(len([]rune(composer.Text)), 0)
	mention := newFileMention()
	if !mention.active(root, composer) {
		t.Fatal("mention picker did not activate")
	}
	select {
	case update := <-mention.updates:
		mention.receive(update)
	case <-time.After(2 * time.Second):
		t.Fatal("project index did not finish within two seconds")
	}
	if !mention.active(root, composer) {
		t.Fatal("mention picker closed after indexing")
	}
	if len(mention.paths) != 1 || mention.paths[0] != "internal/service/server.go" {
		t.Fatalf("global search did not find nested file: %v", mention.paths)
	}
}

func TestEmptyMentionIndexIsNotRepeated(t *testing.T) {
	mention := newFileMention()
	root := t.TempDir()
	mention.ensureIndex(root)
	select {
	case update := <-mention.updates:
		mention.receive(update)
	case <-time.After(2 * time.Second):
		t.Fatal("empty project index did not finish")
	}
	mention.ensureIndex(root)
	if mention.indexing || !mention.indexReady {
		t.Fatal("completed empty index should be cached")
	}
}

func containsMentionPath(paths []string, want string) bool {
	for _, path := range paths {
		if strings.TrimSuffix(path, "/") == want {
			return true
		}
	}
	return false
}
