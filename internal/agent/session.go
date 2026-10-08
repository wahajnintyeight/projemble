package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"projemble/internal/llm"
)

const maxSessionBytes = 16 << 20

type savedSession struct {
	Version    int
	Workspace  string
	SessionID  string
	Title      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Messages   []llm.Message
	Activities []string
	Usage      llm.Usage
	Summary    string
	Undo       []editSnapshot
}

func sessionPath(workspace string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(filepath.Clean(root)))
	return filepath.Join(dir, "projemble", "sessions", hex.EncodeToString(digest[:])+".json"), nil
}

// Restore loads a completed conversation checkpoint. Credentials remain in the provider.
func (agent *Agent) Restore(workspace string) error {
	root, err := filepath.Abs(workspace)
	if err != nil {
		return err
	}
	agent.workspace = root
	index, err := readSessionIndex(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if index.ActiveID != "" {
		return agent.restoreSession(root, index.ActiveID)
	}
	legacyPath, err := sessionPath(root)
	if err != nil {
		return err
	}
	legacy, legacyErr := readSavedSession(legacyPath)
	if legacyErr == nil {
		if err := agent.restoreState(legacy); err != nil {
			return err
		}
		if agent.sessionID == "" {
			agent.sessionID, err = newSessionID()
			if err != nil {
				return err
			}
		}
		return agent.checkpoint()
	}
	if !errors.Is(legacyErr, os.ErrNotExist) {
		return legacyErr
	}
	agent.sessionID, err = newSessionID()
	if err != nil {
		return err
	}
	agent.sessionCreated = time.Now().UTC()
	return agent.checkpoint()
}

func readSavedSession(path string) (savedSession, error) {
	file, err := os.Open(path)
	if err != nil {
		return savedSession{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSessionBytes+1))
	if err != nil {
		return savedSession{}, err
	}
	if len(data) > maxSessionBytes {
		return savedSession{}, errors.New("saved session exceeds 16 MiB")
	}
	var state savedSession
	if err := json.Unmarshal(data, &state); err != nil {
		return savedSession{}, fmt.Errorf("read saved session: %w", err)
	}
	return state, nil
}

func (agent *Agent) restoreSession(root, id string) error {
	path, err := sessionFilePath(root, id)
	if err != nil {
		return err
	}
	state, err := readSavedSession(path)
	if err != nil {
		return err
	}
	if state.SessionID != "" && state.SessionID != id {
		return errors.New("saved session ID mismatch")
	}
	return agent.restoreState(state)
}

func (agent *Agent) restoreState(state savedSession) error {
	if state.Version != 1 || state.Workspace != agent.workspace {
		return errors.New("saved session workspace or version mismatch")
	}
	agent.sessionID, agent.sessionTitle = state.SessionID, state.Title
	agent.sessionCreated, agent.sessionUpdated = state.CreatedAt, state.UpdatedAt
	agent.activities, agent.usage = state.Activities, state.Usage
	agent.undoHistory = restoreUndoHistory(agent.workspace, state.Undo)
	agent.summary, agent.messages = "", nil
	if state.Summary == "" {
		pending := len(state.Messages) > 0 && state.Messages[len(state.Messages)-1].Role == "user"
		pendingUser := lastUserMessage(state.Messages)
		agent.remember(summarizeLegacyMessages(state.Messages))
		agent.messages = nil
		agent.refreshSystemPrompt()
		if pending {
			agent.messages = append(agent.messages, llm.Message{Role: "user", Content: pendingUser})
		}
	} else {
		agent.summary, agent.messages = state.Summary, state.Messages
		agent.refreshSystemPrompt()
	}
	return nil
}

func (agent *Agent) checkpoint() error {
	if agent.sessionID == "" {
		var err error
		agent.sessionID, err = newSessionID()
		if err != nil {
			return err
		}
	}
	if agent.config.APIKey != "" {
		agent.sessionTitle = strings.ReplaceAll(agent.sessionTitle, agent.config.APIKey, "[REDACTED]")
	}
	now := time.Now().UTC()
	if agent.sessionCreated.IsZero() {
		agent.sessionCreated = now
	}
	agent.sessionUpdated = now
	path, err := sessionFilePath(agent.workspace, agent.sessionID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(savedSession{Version: 1, Workspace: agent.workspace, SessionID: agent.sessionID, Title: agent.sessionTitle, CreatedAt: agent.sessionCreated, UpdatedAt: agent.sessionUpdated, Messages: agent.messages, Activities: agent.activities, Usage: agent.usage, Summary: agent.summary, Undo: agent.undoHistory})
	if err != nil {
		return err
	}
	if agent.config.APIKey != "" {
		// JSON escaping must match the encoded secret, including unusual key characters.
		encoded, _ := json.Marshal(agent.config.APIKey)
		data = []byte(strings.ReplaceAll(string(data), string(encoded[1:len(encoded)-1]), "[REDACTED]"))
	}
	if len(data) > maxSessionBytes {
		return errors.New("conversation exceeds 16 MiB; checkpoint was not saved")
	}
	if err := atomicSessionWrite(path, data); err != nil {
		return err
	}
	return updateSessionIndex(agent.workspace, agent.sessionID, SessionInfo{ID: agent.sessionID, Title: agent.sessionTitle, CreatedAt: agent.sessionCreated, UpdatedAt: now})
}

// Transcript supplies user and assistant messages for the reopened workspace.
func (agent *Agent) Usage() llm.Usage { return agent.usage }

func (agent *Agent) Transcript() []string {
	if len(agent.activities) > 0 {
		return append([]string(nil), agent.activities...)
	}
	var rows []string
	for _, message := range agent.messages {
		if message.Role == "user" {
			rows = append(rows, "You: "+message.Content)
		}
		if message.Role == "assistant" && message.Content != "" {
			rows = append(rows, "Agent summary: "+message.Content)
		}
	}
	return rows
}
