package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	maxUndoEntries = 8
	maxUndoBytes   = 4 << 20
	maxDiffBytes   = 6 << 10
)

type editSnapshot struct {
	Path       string
	Existed    bool
	Before     []byte
	BeforeMode uint32
	AfterHash  string
	AfterMode  uint32
}

func createFile(root, name, content string, history *[]editSnapshot) (string, error) {
	if len(content) > maxFileBytes {
		return "", fmt.Errorf("file exceeds %d bytes", maxFileBytes)
	}
	path, err := safePath(root, name)
	if err != nil {
		return "", err
	}
	if isProtectedWritePath(name) {
		return "", errors.New("project instructions and agent control files are protected from automatic edits")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	path, err = safePath(root, name)
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return "", errors.New("file already exists; use edit_file for a targeted change")
	}
	if err != nil {
		return "", err
	}
	if _, err = file.WriteString(content); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", closeErr
	}
	createdInfo, statErr := os.Stat(path)
	if statErr != nil {
		_ = os.Remove(path)
		return "", statErr
	}
	appendUndo(history, editSnapshot{Path: filepath.ToSlash(name), AfterHash: contentHash([]byte(content)), AfterMode: uint32(createdInfo.Mode().Perm())})
	return "Created " + filepath.ToSlash(name), nil
}

func editFile(root, name, oldText, newText string, history *[]editSnapshot) (string, error) {
	if oldText == "" {
		return "", errors.New("old_text must not be empty")
	}
	if len(oldText) > maxFileBytes || len(newText) > maxFileBytes {
		return "", fmt.Errorf("edit text exceeds %d bytes", maxFileBytes)
	}
	path, err := safePath(root, name)
	if err != nil {
		return "", err
	}
	if isProtectedWritePath(name) {
		return "", errors.New("project instructions and agent control files are protected from automatic edits")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return "", fmt.Errorf("edit_file requires a regular file no larger than %d bytes", maxFileBytes)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(before) {
		return "", errors.New("edit_file only supports UTF-8 text files")
	}
	contents := string(before)
	if count := strings.Count(contents, oldText); count != 1 {
		return "", fmt.Errorf("old_text must match exactly once; found %d matches", count)
	}
	index := strings.Index(contents, oldText)
	after := contents[:index] + newText + contents[index+len(oldText):]
	if len(after) > maxFileBytes {
		return "", fmt.Errorf("edited file exceeds %d bytes", maxFileBytes)
	}
	if err := atomicReplace(path, []byte(after), info.Mode().Perm()); err != nil {
		return "", err
	}
	appendUndo(history, editSnapshot{Path: filepath.ToSlash(name), Existed: true, Before: before, BeforeMode: uint32(info.Mode().Perm()), AfterHash: contentHash([]byte(after)), AfterMode: uint32(info.Mode().Perm())})
	line := strings.Count(contents[:index], "\n") + 1
	return compactDiff(name, line, oldText, newText), nil
}

func undoLastEdit(root string, history *[]editSnapshot) (string, error) {
	if history == nil || len(*history) == 0 {
		return "", errors.New("there is no agent edit to undo")
	}
	last := (*history)[len(*history)-1]
	path, err := safePath(root, last.Path)
	if err != nil {
		return "", err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if contentHash(current) != last.AfterHash {
		return "", errors.New("file changed since the agent edit; refusing to overwrite newer work")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if uint32(info.Mode().Perm()) != last.AfterMode {
		return "", errors.New("file permissions changed since the agent edit; refusing to overwrite newer work")
	}
	if last.Existed {
		if err := atomicReplace(path, last.Before, os.FileMode(last.BeforeMode)); err != nil {
			return "", err
		}
	} else if err := os.Remove(path); err != nil {
		return "", err
	}
	*history = (*history)[:len(*history)-1]
	if last.Existed {
		return "Undid edit to " + last.Path, nil
	}
	return "Removed created file " + last.Path, nil
}

func atomicReplace(path string, contents []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".projemble-edit-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func appendUndo(history *[]editSnapshot, snapshot editSnapshot) {
	if history == nil {
		return
	}
	*history = append(*history, snapshot)
	total := 0
	for _, item := range *history {
		total += len(item.Before) + len(item.Path) + len(item.AfterHash)
	}
	for len(*history) > maxUndoEntries || total > maxUndoBytes {
		oldest := (*history)[0]
		total -= len(oldest.Before) + len(oldest.Path) + len(oldest.AfterHash)
		*history = (*history)[1:]
	}
}

func restoreUndoHistory(root string, snapshots []editSnapshot) []editSnapshot {
	if len(snapshots) > maxUndoEntries {
		snapshots = snapshots[len(snapshots)-maxUndoEntries:]
	}
	var history []editSnapshot
	for _, snapshot := range snapshots {
		if len(snapshot.Before) > maxFileBytes || (snapshot.Existed == false && len(snapshot.Before) != 0) {
			continue
		}
		if _, err := hex.DecodeString(snapshot.AfterHash); err != nil || len(snapshot.AfterHash) != sha256.Size*2 {
			continue
		}
		if _, err := safePath(root, snapshot.Path); err != nil {
			continue
		}
		history = append(history, snapshot)
	}
	for undoBytes(history) > maxUndoBytes && len(history) > 0 {
		history = history[1:]
	}
	return history
}

func undoBytes(history []editSnapshot) int {
	total := 0
	for _, item := range history {
		total += len(item.Before) + len(item.Path) + len(item.AfterHash)
	}
	return total
}

func contentHash(contents []byte) string {
	hash := sha256.Sum256(contents)
	return hex.EncodeToString(hash[:])
}

func compactDiff(name string, line int, oldText, newText string) string {
	name = filepath.ToSlash(name)
	return fmt.Sprintf("Updated %s\n--- a/%s\n+++ b/%s\n@@ line %d @@\n%s%s", name, name, name, line, diffLines('-', oldText), diffLines('+', newText))
}

func diffLines(prefix byte, text string) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	var result strings.Builder
	for _, line := range lines {
		remaining := maxDiffBytes - result.Len() - 2
		if remaining <= 0 {
			result.WriteString("...\n")
			break
		}
		line = truncateUTF8(line, remaining)
		result.WriteByte(prefix)
		result.WriteString(line)
		result.WriteByte('\n')
		if result.Len() >= maxDiffBytes {
			result.WriteString("...\n")
			break
		}
	}
	return result.String()
}
