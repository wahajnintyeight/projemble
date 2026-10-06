package projectstore

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"projemble/internal/catalog"
)

const (
	currentVersion = 1
	maxConfigBytes = 1 << 20
)

type Config struct {
	Version  int       `yaml:"version"`
	Projects []Project `yaml:"projects"`
}

type Project struct {
	ID             string            `yaml:"id"`
	Name           string            `yaml:"name"`
	Description    string            `yaml:"description,omitempty"`
	Path           string            `yaml:"path"`
	StackID        string            `yaml:"stack"`
	AppShapeID     string            `yaml:"app_shape"`
	ArchitectureID string            `yaml:"architecture"`
	TemplateID     string            `yaml:"template"`
	Settings       map[string]string `yaml:"settings,omitempty"`
	CreatedAt      string            `yaml:"created_at"`
	UpdatedAt      string            `yaml:"updated_at"`
}

func NewConfig() Config {
	return Config{Version: currentVersion, Projects: []Project{}}
}

func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()

	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(configDir, "projemble", "config.yaml"), nil
}

func LoadDefault() (Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, err
	}
	return Load(path)
}

func SaveDefault(config Config) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return Save(path, config)
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewConfig(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	if len(data) > maxConfigBytes {
		return Config{}, fmt.Errorf("config %q exceeds %d bytes", path, maxConfigBytes)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return NewConfig(), nil
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}
	if config.Version == 0 {
		config.Version = currentVersion
	}
	if config.Projects == nil {
		config.Projects = []Project{}
	}
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}
	return config, nil
}

func Save(path string, config Config) error {
	if config.Version == 0 {
		config.Version = currentVersion
	}
	if config.Projects == nil {
		config.Projects = []Project{}
	}
	if err := config.Validate(); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if len(data) > maxConfigBytes {
		return fmt.Errorf("encoded config exceeds %d bytes", maxConfigBytes)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := writeReplace(path, data); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

func writeReplace(path string, data []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".projemble-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func (config *Config) Upsert(project Project) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if project.ID == "" {
		id, err := newID()
		if err != nil {
			return err
		}
		project.ID = id
	}
	if project.CreatedAt == "" {
		project.CreatedAt = now
	}
	project.UpdatedAt = now

	for i := range config.Projects {
		if config.Projects[i].ID == project.ID {
			if project.CreatedAt == now && config.Projects[i].CreatedAt != "" {
				project.CreatedAt = config.Projects[i].CreatedAt
			}
			config.Projects[i] = project
			return config.Validate()
		}
	}
	config.Projects = append(config.Projects, project)
	if config.Version == 0 {
		config.Version = currentVersion
	}
	return config.Validate()
}

func (config Config) Validate() error {
	if config.Version != currentVersion {
		return fmt.Errorf("unsupported config version %d", config.Version)
	}
	seen := make(map[string]struct{}, len(config.Projects))
	for i, project := range config.Projects {
		prefix := fmt.Sprintf("projects[%d]", i)
		if strings.TrimSpace(project.ID) == "" {
			return fmt.Errorf("%s: id is required", prefix)
		}
		if _, exists := seen[project.ID]; exists {
			return fmt.Errorf("%s: duplicate id %q", prefix, project.ID)
		}
		seen[project.ID] = struct{}{}
		if strings.TrimSpace(project.Name) == "" {
			return fmt.Errorf("%s: name is required", prefix)
		}
		if strings.TrimSpace(project.Path) == "" {
			return fmt.Errorf("%s: path is required", prefix)
		}
		template, ok := catalog.TemplateByID(project.TemplateID)
		if !ok {
			return fmt.Errorf("%s: unknown template %q", prefix, project.TemplateID)
		}
		if project.StackID != template.StackID ||
			project.AppShapeID != template.AppShapeID ||
			project.ArchitectureID != template.ArchitectureID {
			return fmt.Errorf("%s: stack, app shape, and architecture must match template %q", prefix, project.TemplateID)
		}
	}
	return nil
}

func newID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("generate project id: %w", err)
	}
	return hex.EncodeToString(id[:]), nil
}
