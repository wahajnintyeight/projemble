package tui

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxIndexedEntries = 30000
	mentionReadBatch  = 256
)

type mentionIndexUpdate struct {
	root       string
	generation uint64
	paths      []string
	truncated  bool
	err        error
}

func scanMentionTree(ctx context.Context, root string, limit int) ([]string, bool, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, false, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, false, err
	}
	if !info.IsDir() {
		return nil, false, fs.ErrInvalid
	}
	paths := make([]string, 0, min(limit, 4096))
	directories := []string{root}
	visited := 0
	for len(directories) > 0 {
		if err := ctx.Err(); err != nil {
			return paths, false, err
		}
		last := len(directories) - 1
		directory := directories[last]
		directories = directories[:last]
		dir, openErr := os.Open(directory)
		if openErr != nil {
			continue
		}
		for {
			entries, readErr := dir.ReadDir(mentionReadBatch)
			for _, entry := range entries {
				visited++
				if visited > limit {
					dir.Close()
					sort.Strings(paths)
					return paths, true, nil
				}
				name := entry.Name()
				if name == "" {
					continue
				}
				full := filepath.Join(directory, name)
				relative, relErr := filepath.Rel(root, full)
				if relErr != nil {
					continue
				}
				path := filepath.ToSlash(relative)
				if entry.IsDir() {
					if ignoredMentionDirectory(name) {
						continue
					}
					path += "/"
					paths = append(paths, path)
					directories = append(directories, full)
				} else if entry.Type()&fs.ModeSymlink == 0 {
					paths = append(paths, path)
				}
				if len(paths) >= limit {
					dir.Close()
					sort.Strings(paths)
					return paths, true, nil
				}
			}
			if readErr == io.EOF || len(entries) == 0 {
				break
			}
			if readErr != nil {
				break
			}
			if err := ctx.Err(); err != nil {
				dir.Close()
				return paths, false, err
			}
		}
		dir.Close()
	}
	sort.Strings(paths)
	return paths, false, nil
}

func ignoredMentionDirectory(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", "vendor", ".next", ".nuxt", "dist", "build", "target", ".venv", "venv", "coverage":
		return true
	default:
		return false
	}
}

func matchMentionPath(path, query string) (int, bool) {
	query = strings.ToLower(strings.TrimSpace(query))
	candidate := strings.ToLower(strings.TrimSuffix(path, "/"))
	if query == "" {
		return 0, true
	}
	base := strings.ToLower(filepath.Base(filepath.FromSlash(candidate)))
	if strings.HasPrefix(base, query) {
		return 0, true
	}
	if strings.Contains(base, query) {
		return 1, true
	}
	if strings.HasPrefix(candidate, query) {
		return 2, true
	}
	if strings.Contains(candidate, query) {
		return 3, true
	}
	queryRunes := []rune(query)
	qi := 0
	gaps := 0
	last := -1
	for i, r := range candidate {
		if qi < len(queryRunes) && r == queryRunes[qi] {
			if last >= 0 {
				gaps += i - last - 1
			}
			last = i
			qi++
		}
	}
	if qi < len(queryRunes) {
		return 0, false
	}
	return 10 + gaps, true
}

func rankMentionPaths(paths []string, query string, limit int) []string {
	type ranked struct {
		path  string
		score int
	}
	values := make([]ranked, 0, min(len(paths), limit*4))
	for _, path := range paths {
		if score, ok := matchMentionPath(path, query); ok {
			values = append(values, ranked{path, score})
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].score != values[j].score {
			return values[i].score < values[j].score
		}
		if len(values[i].path) != len(values[j].path) {
			return len(values[i].path) < len(values[j].path)
		}
		return values[i].path < values[j].path
	})
	if len(values) > limit {
		values = values[:limit]
	}
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = value.path
	}
	return result
}
