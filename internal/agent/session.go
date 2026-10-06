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

	"projemble/internal/llm"
)

const maxSessionBytes = 16 << 20

type savedSession struct {
	Version    int
	Workspace  string
	Messages   []llm.Message
	Activities []string
	Usage      llm.Usage
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
	path, err := sessionPath(workspace)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSessionBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxSessionBytes {
		return errors.New("saved session exceeds 16 MiB")
	}
	var state savedSession
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("read saved session: %w", err)
	}
	root, err := filepath.Abs(workspace)
	if err != nil {
		return err
	}
	if state.Version != 1 || state.Workspace != root {
		return errors.New("saved session workspace or version mismatch")
	}
	agent.workspace, agent.messages = root, state.Messages
	agent.activities, agent.usage = state.Activities, state.Usage
	return nil
}

func (agent *Agent) checkpoint() error {
	path, err := sessionPath(agent.workspace)
	if err != nil {
		return err
	}
	data, err := json.Marshal(savedSession{Version: 1, Workspace: agent.workspace, Messages: agent.messages, Activities: agent.activities, Usage: agent.usage})
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
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".session-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
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
