package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const DefaultFileName = "divinity.yaml"

type Config struct {
	Version     int               `yaml:"version"`
	Agents      []AgentConfig     `yaml:"agents"`
	Validation  []ValidationCheck `yaml:"validation"`
	Preferences Preferences       `yaml:"preferences"`
}

type AgentConfig struct {
	Name            string            `yaml:"name"`
	Type            string            `yaml:"type"`
	Command         string            `yaml:"command"`
	Args            []string          `yaml:"args"`
	Env             map[string]string `yaml:"env"`
	Description     string            `yaml:"description"`
	BaseURL         string            `yaml:"base_url"`
	Model           string            `yaml:"model"`
	APIKeyEnv       string            `yaml:"api_key_env"`
	System          string            `yaml:"system"`
	OutputFile      string            `yaml:"output_file"`
	Temperature     *float64          `yaml:"temperature"`
	MaxSteps        int               `yaml:"max_steps"`
	AllowedCommands []string          `yaml:"allowed_commands"`
}

type ValidationCheck struct {
	Name    string   `yaml:"name"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type Preferences struct {
	MaxParallelAgents int `yaml:"max_parallel_agents"`
}

func Default() Config {
	return Config{
		Version: 1,
		Agents: []AgentConfig{
			{
				Name:        "echo-plan",
				Type:        "shell",
				Command:     "divinity-example-agent",
				Args:        []string{"{{task}}"},
				Description: "Replace this with a real CLI agent command such as Gemini CLI, Claude Code, Aider, or a custom script.",
			},
		},
		Validation: []ValidationCheck{
			{Name: "git-status", Command: "git", Args: []string{"status", "--short"}},
		},
		Preferences: Preferences{MaxParallelAgents: 4},
	}
}

func WriteDefault(path string) error {
	return Write(path, Default())
}

func Write(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func Load(path string) (Config, string, error) {
	cfg, root, err := LoadAllowMissing(path)
	if err != nil {
		return Config{}, "", err
	}
	if len(cfg.Agents) == 0 {
		return Config{}, "", fmt.Errorf("no agents configured in %s", filepath.Join(root, DefaultFileName))
	}
	return cfg, root, nil
}

func LoadAllowMissing(path string) (Config, string, error) {
	root, err := os.Getwd()
	if err != nil {
		return Config{}, "", err
	}

	if path == "" {
		path = filepath.Join(root, DefaultFileName)
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), root, nil
	}
	if err != nil {
		return Config{}, "", err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, "", err
	}

	if cfg.Preferences.MaxParallelAgents <= 0 {
		cfg.Preferences.MaxParallelAgents = 4
	}

	return cfg, root, nil
}

func (c Config) SelectAgents(names []string) ([]AgentConfig, error) {
	if len(names) == 0 {
		return c.Agents, nil
	}

	byName := make(map[string]AgentConfig, len(c.Agents))
	for _, agent := range c.Agents {
		byName[agent.Name] = agent
	}

	selected := make([]AgentConfig, 0, len(names))
	for _, name := range names {
		agent, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("agent %q is not configured", name)
		}
		selected = append(selected, agent)
	}
	return selected, nil
}
