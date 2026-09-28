// Package config manages the global registry of brains (the user-level index of
// where each brain lives on disk and which one is currently active) and of
// repository checkouts on this machine.
package config

import (
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// Brain is a registry entry pointing at a brain repository on disk. Aliases
// are extra names it resolves by; anything multi writes stores Name.
type Brain struct {
	Name    string   `yaml:"name"`
	Path    string   `yaml:"path"`
	Aliases []string `yaml:"aliases,omitempty,flow"`
}

// Repo is a registry entry pointing at a repository checkout on this machine.
// Only the path is stored; the remote is inferred live from the checkout.
type Repo struct {
	Path    string   `yaml:"path"`
	Aliases []string `yaml:"aliases,omitempty,flow"`
}

// UnmarshalYAML also accepts the v1.6 form, a plain path string, so older
// configs keep loading.
func (r *Repo) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		r.Path = n.Value
		return nil
	}
	type plain Repo
	return n.Decode((*plain)(r))
}

// Config is the user-level registry, stored at ~/.config/multi/config.yaml.
type Config struct {
	Active string  `yaml:"active,omitempty"`
	Brains []Brain `yaml:"brains,omitempty"`
	// Platforms maps a git host to the prefix of the repo ids derived from it.
	Platforms map[string]string `yaml:"platforms,omitempty"`
	// Repos maps a repository id to this machine's checkout.
	Repos map[string]Repo `yaml:"repos,omitempty"`

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
	// a hand-edited alias in active resolves like a typed one and is saved as the name
	if ab := c.ActiveBrain(); ab != nil {
		c.Active = ab.Name
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

// Find returns the registry entry with the given name or, failing that, alias,
// or nil. Every brain reference (--brain, .multi.yaml, active) resolves here.
func (c *Config) Find(name string) *Brain {
	for i := range c.Brains {
		if c.Brains[i].Name == name {
			return &c.Brains[i]
		}
	}
	return c.AliasOwner(name)
}

// AliasOwner returns the registry entry carrying alias, or nil.
func (c *Config) AliasOwner(alias string) *Brain {
	for i := range c.Brains {
		if slices.Contains(c.Brains[i].Aliases, alias) {
			return &c.Brains[i]
		}
	}
	return nil
}

// Add registers a brain, replacing the path of an existing entry with the same
// name. Callers keep names and aliases apart.
func (c *Config) Add(b Brain) {
	if existing := c.Find(b.Name); existing != nil && existing.Name == b.Name {
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
