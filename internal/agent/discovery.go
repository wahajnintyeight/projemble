package agent

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxReadFileOutput = 24 << 10
	maxSearchBytes    = 16 << 20
	maxSearchFiles    = 1_000
	maxSearchHits     = 100
	maxSearchLine     = 240
)

func decodeReadFileArguments(raw string) (map[string]string, error) {
	values, err := decodeToolObject(raw, []string{"path", "start_line", "end_line"})
	if err != nil {
		return nil, err
	}
	args := make(map[string]string, len(values))
	for _, field := range []string{"path", "start_line", "end_line"} {
		value, ok := values[field]
		if !ok {
			continue
		}
		if field == "path" {
			args[field], err = decodeToolString(value, field)
			if err == nil && strings.TrimSpace(args[field]) == "" {
				err = fmt.Errorf("required tool argument %q is missing or empty", field)
			}
		} else {
			var line int
			if err = json.Unmarshal(value, &line); err == nil {
				if line < 1 {
					err = fmt.Errorf("tool argument %q must be a positive line number", field)
				} else {
					args[field] = strconv.Itoa(line)
				}
			} else {
				err = fmt.Errorf("tool argument %q must be an integer", field)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	if _, ok := args["path"]; !ok {
		return nil, fmt.Errorf("required tool argument %q is missing or empty", "path")
	}
	startLine, endLine := 1, 0
	if value, ok := args["start_line"]; ok {
		startLine, _ = strconv.Atoi(value)
	}
	if value, ok := args["end_line"]; ok {
		endLine, _ = strconv.Atoi(value)
	}
	if endLine > 0 && endLine-startLine > 999 {
		return nil, fmt.Errorf("read_file ranges are limited to 1,000 lines")
	}
	if start, ok := args["start_line"]; ok {
		startLine, _ := strconv.Atoi(start)
		endLine, parseErr := strconv.Atoi(args["end_line"])
		if parseErr == nil && endLine < startLine {
			return nil, fmt.Errorf("end_line must be greater than or equal to start_line")
		}
	}
	return args, nil
}

func decodeSearchArguments(raw string) (map[string]string, error) {
	values, err := decodeToolObject(raw, []string{"query", "path"})
	if err != nil {
		return nil, err
	}
	args := map[string]string{"path": "."}
	for _, field := range []string{"query", "path"} {
		value, ok := values[field]
		if !ok {
			continue
		}
		args[field], err = decodeToolString(value, field)
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(args["query"]) == "" {
		return nil, fmt.Errorf("required tool argument %q is missing or empty", "query")
	}
	if len(args["query"]) > 1_024 || strings.ContainsAny(args["query"], "\r\n") {
		return nil, fmt.Errorf("search query must contain 1 to 1,024 bytes on one line")
	}
	if strings.TrimSpace(args["path"]) == "" {
		return nil, fmt.Errorf("search path must be a relative directory or omitted")
	}
	return args, nil
}

func parseOptionalLine(value string) int {
	line, _ := strconv.Atoi(value)
	return line
}

func readFileRange(root, name string, start, end int) (string, error) {
	path, err := safePath(root, name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("only regular project files can be read")
	}
	if info.Size() > maxFileBytes {
		return "", fmt.Errorf("file exceeds %d bytes", maxFileBytes)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(contents) || strings.IndexByte(string(contents), 0) >= 0 {
		return "", fmt.Errorf("file is not UTF-8 text")
	}
	lines := strings.Split(string(contents), "\n")
	if start < 1 {
		start = 1
	}
	if end < 1 {
		end = min(start+199, len(lines))
	}
	if end-start > 999 {
		return "", fmt.Errorf("read_file ranges are limited to 1,000 lines")
	}
	if start > len(lines) {
		return "", fmt.Errorf("start_line %d is past the end of the file (%d lines)", start, len(lines))
	}
	end = min(end, len(lines))
	var result strings.Builder
	shownEnd := start - 1
	for index := start - 1; index < end; index++ {
		line := strings.TrimSuffix(lines[index], "\r")
		if len(line) > 4<<10 {
			line = truncateUTF8(line, 4<<10) + " [line truncated]"
		}
		formatted := fmt.Sprintf("%5d | %s\n", index+1, line)
		if result.Len()+len(formatted) > maxReadFileOutput {
			break
		}
		result.WriteString(formatted)
		shownEnd = index + 1
	}
	if shownEnd < end || end < len(lines) {
		next := shownEnd + 1
		if next < start {
			next = start
		}
		fmt.Fprintf(&result, "[showing lines %d-%d of %d; continue with start_line=%d]\n", start, shownEnd, len(lines), next)
	}
	return result.String(), nil
}

func searchFiles(root, name, query string) (string, error) {
	if strings.TrimSpace(query) == "" || len(query) > 1_024 || strings.ContainsAny(query, "\r\n") {
		return "", fmt.Errorf("search query must contain 1 to 1,024 bytes on one line")
	}
	if name == "" {
		name = "."
	}
	path, err := safePath(root, name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("search path must be a project directory")
	}
	needle := strings.ToLower(query)
	var result strings.Builder
	files, entries, bytesRead, hits := 0, 0, int64(0), 0
	limitedFiles, limitedEntries, limitedBytes, limitedHits := false, false, false, false
	err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > maxDirectoryEntries {
			limitedEntries = true
			return filepath.SkipAll
		}
		if entry.IsDir() && current != path {
			rel, relErr := filepath.Rel(root, current)
			if skipProjectDirectory(entry.Name()) || (relErr == nil && (isCredentialPath(rel) || isPrivateControlPath(rel))) {
				return filepath.SkipDir
			}
		}
		if entry.Type()&fs.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, current)
		if err != nil || isCredentialPath(rel) || isPrivateControlPath(rel) {
			return nil
		}
		files++
		if files > maxSearchFiles {
			limitedFiles = true
			return filepath.SkipAll
		}
		fileInfo, err := entry.Info()
		if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Size() > maxFileBytes {
			return nil
		}
		if bytesRead+fileInfo.Size() > maxSearchBytes {
			limitedBytes = true
			return filepath.SkipAll
		}
		data, err := os.ReadFile(current)
		if err != nil {
			return nil
		}
		bytesRead += int64(len(data))
		if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
			return nil
		}
		for lineNo, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(strings.ToLower(line), needle) {
				continue
			}
			hits++
			if hits > maxSearchHits {
				limitedHits = true
				return filepath.SkipAll
			}
			line = strings.TrimSuffix(line, "\r")
			fmt.Fprintf(&result, "%s:%d: %s\n", filepath.ToSlash(rel), lineNo+1, truncateUTF8(line, maxSearchLine))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if result.Len() == 0 {
		result.WriteString("No matches found.\n")
	}
	if limitedHits {
		result.WriteString("[stopped after 100 matches]\n")
	}
	if limitedFiles {
		result.WriteString("[stopped after 1,000 files]\n")
	}
	if limitedEntries {
		result.WriteString("[stopped after 10,000 directory entries]\n")
	}
	if limitedBytes {
		result.WriteString("[stopped after scanning 16 MiB]\n")
	}
	return result.String(), nil
}

func skipProjectDirectory(name string) bool {
	switch name {
	case ".git", ".projemble", ".codex", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	cut := maxBytes
	for cut > 0 && !utf8.ValidString(value[:cut]) {
		cut--
	}
	return value[:cut] + "…"
}
