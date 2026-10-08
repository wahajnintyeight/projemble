package agent

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"projemble/internal/llm"
)

const maxSessionIndexBytes = 4 << 20

type SessionInfo struct {
	ID        string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type sessionIndex struct {
	Version  int
	ActiveID string
	Sessions []SessionInfo
}

func newSessionID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("create session ID: %w", err)
	}
	return hex.EncodeToString(id[:]), nil
}

func sessionPaths(workspace string) (string, string, string, error) {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return "", "", "", err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256([]byte(filepath.Clean(root)))
	key := hex.EncodeToString(digest[:])
	base := filepath.Join(configDir, "projemble", "sessions")
	return filepath.Join(base, key), filepath.Join(base, key+".index.json"), base, nil
}

func sessionFilePath(workspace, id string) (string, error) {
	if len(id) != 32 {
		return "", errors.New("invalid session ID")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return "", errors.New("invalid session ID")
	}
	dir, _, _, err := sessionPaths(workspace)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, id+".json"), nil
}

func readSessionIndex(workspace string) (sessionIndex, error) {
	_, path, _, err := sessionPaths(workspace)
	if err != nil {
		return sessionIndex{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return sessionIndex{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSessionIndexBytes+1))
	if err != nil {
		return sessionIndex{}, err
	}
	if len(data) > maxSessionIndexBytes {
		return sessionIndex{}, errors.New("session index exceeds 4 MiB")
	}
	var index sessionIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return sessionIndex{}, fmt.Errorf("read session index: %w", err)
	}
	if index.Version != 1 {
		return sessionIndex{}, errors.New("unsupported session index version")
	}
	return index, nil
}

func updateSessionIndex(workspace, activeID string, info SessionInfo) error {
	dir, indexPath, _, err := sessionPaths(workspace)
	if err != nil {
		return err
	}
	index, err := readSessionIndex(workspace)
	if errors.Is(err, os.ErrNotExist) {
		index = sessionIndex{Version: 1}
	} else if err != nil {
		return err
	}
	index.ActiveID = activeID
	updated := false
	for i := range index.Sessions {
		if index.Sessions[i].ID == info.ID {
			index.Sessions[i] = info
			updated = true
			break
		}
	}
	if !updated {
		index.Sessions = append(index.Sessions, info)
	}
	sort.Slice(index.Sessions, func(i, j int) bool {
		return index.Sessions[i].UpdatedAt.After(index.Sessions[j].UpdatedAt)
	})
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	if len(data) > maxSessionIndexBytes {
		return errors.New("session index exceeds 4 MiB")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return atomicSessionWrite(indexPath, data)
}

func atomicSessionWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".session-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
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

func (agent *Agent) SessionID() string { return agent.sessionID }

func (agent *Agent) Sessions(workspace string) ([]SessionInfo, error) {
	index, err := readSessionIndex(workspace)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return append([]SessionInfo(nil), index.Sessions...), nil
}

func (agent *Agent) StartNewSession(carrySummary bool) error {
	if agent.workspace == "" {
		return errors.New("agent workspace is not set")
	}
	if err := agent.checkpoint(); err != nil {
		return fmt.Errorf("save current session: %w", err)
	}
	previousID := agent.sessionID
	summary := ""
	if carrySummary {
		summary = agent.summary
	}
	id, err := newSessionID()
	if err != nil {
		return err
	}
	agent.sessionID, agent.sessionTitle = id, ""
	agent.sessionCreated, agent.sessionUpdated = time.Now().UTC(), time.Time{}
	agent.messages, agent.activities, agent.undoHistory = nil, nil, nil
	agent.usage, agent.summary = llm.Usage{}, summary
	agent.refreshSystemPrompt()
	if err := agent.checkpoint(); err != nil {
		_ = agent.restoreSession(agent.workspace, previousID)
		return fmt.Errorf("save new session: %w", err)
	}
	return nil
}

func (agent *Agent) ResumeSession(workspace, requestedID string) error {
	if agent.workspace == "" {
		return errors.New("agent workspace is not set")
	}
	root, err := filepath.Abs(workspace)
	if err != nil {
		return err
	}
	if root != agent.workspace {
		return errors.New("agent session cannot change workspaces")
	}
	sessions, err := agent.Sessions(root)
	if err != nil {
		return err
	}
	requestedID = strings.ToLower(strings.TrimSpace(requestedID))
	if len(requestedID) < 8 || len(requestedID) > 32 {
		return errors.New("session ID must be at least 8 hexadecimal characters")
	}
	if _, err := hex.DecodeString(requestedID); err != nil {
		return errors.New("session ID must be hexadecimal")
	}
	match := ""
	for _, session := range sessions {
		if strings.HasPrefix(session.ID, requestedID) {
			if match != "" {
				return errors.New("session ID prefix is ambiguous; enter more characters")
			}
			match = session.ID
		}
	}
	if match == "" {
		return errors.New("saved session not found; run /sessions to see available IDs")
	}
	if match == agent.sessionID {
		return nil
	}
	previousID := agent.sessionID
	if err := agent.checkpoint(); err != nil {
		return fmt.Errorf("save current session: %w", err)
	}
	if err := agent.restoreSession(root, match); err != nil {
		return err
	}
	if err := updateSessionIndex(root, match, SessionInfo{ID: agent.sessionID, Title: agent.sessionTitle, CreatedAt: agent.sessionCreated, UpdatedAt: agent.sessionUpdated}); err != nil {
		_ = agent.restoreSession(root, previousID)
		return err
	}
	return nil
}
