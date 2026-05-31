// Package config manages the global registry of brains (the user-level index of
// where each brain lives on disk and which one is currently active).
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Brain is a registry entry pointing at a brain repository on disk.
type Brain struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// Config is the user-level registry, stored at ~/.config/multi/config.yaml.
type Config struct {
	Active string  `yaml:"active,omitempty"`
	Brains []Brain `yaml:"brains,omitempty"`

	path string `yaml:"-"`
}

// Dir is the directory holding the registry. Overridable via MULTI_CONFIG_DIR
// (primarily for tests and isolated environments).
func Dir() string {
	if d := os.Getenv("MULTI_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".multi"
	}
	return filepath.Join(home, ".config", "multi")
}

// Load reads the registry, returning an empty (but usable) config if none exists.
func Load() (*Config, error) {
	p := filepath.Join(Dir(), "config.yaml")
	c := &Config{path: p}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	c.path = p
	return c, nil
}

// Save persists the registry, creating the config directory as needed.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o644)
}

// Find returns the registry entry with the given name, or nil.
func (c *Config) Find(name string) *Brain {
	for i := range c.Brains {
		if c.Brains[i].Name == name {
			return &c.Brains[i]
		}
	}
	return nil
}

// Add registers a brain, replacing the path of an existing entry with the same name.
func (c *Config) Add(b Brain) {
	if existing := c.Find(b.Name); existing != nil {
		existing.Path = b.Path
		return
	}
	c.Brains = append(c.Brains, b)
}

// ActiveBrain returns the currently selected registry entry, or nil.
func (c *Config) ActiveBrain() *Brain {
	if c.Active == "" {
		return nil
	}
	return c.Find(c.Active)
}
