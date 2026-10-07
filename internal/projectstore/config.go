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
	LastProjectID   string             `yaml:"last_project_id,omitempty"`
	ParentDirectory string             `yaml:"parent_directory,omitempty"`
	Version         int                `yaml:"version"`
	AgentAccessMode string             `yaml:"agent_access_mode,omitempty"`
	Generation      GenerationDefaults `yaml:"generation,omitempty"`
	ProviderKeys    map[string]string  `yaml:"provider_keys,omitempty"`
	Projects        []Project          `yaml:"projects"`
}

type GenerationDefaults struct {
	Mode            string `yaml:"mode,omitempty"`
	Provider        string `yaml:"provider,omitempty"`
	Model           string `yaml:"model,omitempty"`
	ReasoningEffort string `yaml:"reasoning_effort,omitempty"`
}

type Project struct {
	Status          string            `yaml:"status,omitempty"`
	ID              string            `yaml:"id"`
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description,omitempty"`
	Path            string            `yaml:"path"`
	StackID         string            `yaml:"stack"`
	AppShapeID      string            `yaml:"app_shape"`
	WorkloadID      string            `yaml:"workload,omitempty"`
	TopologyID      string            `yaml:"topology,omitempty"`
	ArchitectureID  string            `yaml:"architecture"`
	TemplateID      string            `yaml:"template"`
	PatternIDs      []string          `yaml:"patterns,omitempty"`
	Capabilities    []string          `yaml:"capabilities,omitempty"`
	GenerationMode  string            `yaml:"generation_mode,omitempty"`
	AIProvider      string            `yaml:"ai_provider,omitempty"`
	AIModel         string            `yaml:"ai_model,omitempty"`
	ReasoningEffort string            `yaml:"reasoning_effort,omitempty"`
	Settings        map[string]string `yaml:"settings,omitempty"`
	CreatedAt       string            `yaml:"created_at"`
	UpdatedAt       string            `yaml:"updated_at"`
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
	for i := range config.Projects {
		config.Projects[i] = NormalizeProject(config.Projects[i])
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
	project = NormalizeProject(project)
	now := time.Now().UTC().Format(time.RFC3339)
	if project.ID == "" {
		for _, existing := range config.Projects {
			if filepath.Clean(existing.Path) == filepath.Clean(project.Path) {
				project.ID = existing.ID
				project.CreatedAt = existing.CreatedAt
				break
			}
		}
	}
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
	if project.GenerationMode == "" {
		project.GenerationMode = "local"
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

// NormalizeProject fills fields added after config v1 using its stable template ID.
func NormalizeProject(project Project) Project {
	if template, ok := catalog.TemplateByID(project.TemplateID); ok {
		if project.StackID == "" {
			project.StackID = template.StackID
		}
		if project.AppShapeID == "" {
			project.AppShapeID = template.AppShapeID
		}
		if project.WorkloadID == "" {
			project.WorkloadID = template.WorkloadID
		}
		if project.TopologyID == "" {
			project.TopologyID = template.TopologyID
		}
		if project.ArchitectureID == "" {
			project.ArchitectureID = template.ArchitectureID
		}
	}
	if project.PatternIDs == nil {
		project.PatternIDs = []string{}
	}
	if project.Capabilities == nil {
		project.Capabilities = []string{}
	}
	return project
}

func (config Config) Validate() error {
	if config.Version != currentVersion {
		return fmt.Errorf("unsupported config version %d", config.Version)
	}
	if config.Generation.Mode != "" && config.Generation.Mode != "local" && config.Generation.Mode != "agent" {
		return fmt.Errorf("unknown default generation mode %q", config.Generation.Mode)
	}
	if config.Generation.Mode == "agent" && (strings.TrimSpace(config.Generation.Provider) == "" || strings.TrimSpace(config.Generation.Model) == "") {
		return errors.New("default AI provider and model are required for agent generation")
	}
	if config.AgentAccessMode != "" && config.AgentAccessMode != "read-only" && config.AgentAccessMode != "full-access" && config.AgentAccessMode != "ask-always" {
		return fmt.Errorf("unknown agent access mode %q", config.AgentAccessMode)
	}
	if !validReasoningEffort(config.Generation.ReasoningEffort) {
		return fmt.Errorf("unknown reasoning effort %q", config.Generation.ReasoningEffort)
	}
	seen := make(map[string]struct{}, len(config.Projects))
	for i, project := range config.Projects {
		project = NormalizeProject(project)
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
		if !validReasoningEffort(project.ReasoningEffort) {
			return fmt.Errorf("%s: unknown reasoning effort %q", prefix, project.ReasoningEffort)
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
			project.WorkloadID != template.WorkloadID ||
			project.TopologyID != template.TopologyID ||
			project.ArchitectureID != template.ArchitectureID {
			return fmt.Errorf("%s: stack, workload, topology, app shape, and architecture must match template %q", prefix, project.TemplateID)
		}
		patterns := make(map[string]struct{}, len(project.PatternIDs))
		for _, id := range project.PatternIDs {
			if _, ok := catalog.PatternByID(id); !ok {
				return fmt.Errorf("%s: unknown application pattern %q", prefix, id)
			}
			if _, exists := patterns[id]; exists {
				return fmt.Errorf("%s: duplicate application pattern %q", prefix, id)
			}
			patterns[id] = struct{}{}
		}
		capabilities := make(map[string]struct{}, len(project.Capabilities))
		primaryDatabase := false
		for _, id := range project.Capabilities {
			capability, ok := catalog.CapabilityByID(id)
			if !ok {
				return fmt.Errorf("%s: unknown capability %q", prefix, id)
			}
			if !capability.Supported {
				return fmt.Errorf("%s: capability %q is planned but not supported yet", prefix, id)
			}
			if _, exists := capabilities[id]; exists {
				return fmt.Errorf("%s: duplicate capability %q", prefix, id)
			}
			if capability.Category == "Primary database" && primaryDatabase {
				return fmt.Errorf("%s: choose at most one primary database", prefix)
			}
			primaryDatabase = primaryDatabase || capability.Category == "Primary database"
			capabilities[id] = struct{}{}
		}
		if project.GenerationMode != "" && project.GenerationMode != "local" && project.GenerationMode != "agent" {
			return fmt.Errorf("%s: unknown generation mode %q", prefix, project.GenerationMode)
		}
		if project.GenerationMode == "agent" && (strings.TrimSpace(project.AIProvider) == "" || strings.TrimSpace(project.AIModel) == "") {
			return fmt.Errorf("%s: AI provider and model are required for agent generation", prefix)
		}
	}
	return nil
}

func validReasoningEffort(effort string) bool {
	switch effort {
	case "", "low", "medium", "high":
		return true
	default:
		return false
	}
}

func newID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("generate project id: %w", err)
	}
	return hex.EncodeToString(id[:]), nil
}
