package globalconfig

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the global m2a configuration stored at %APPDATA%\m2a\m2a.yml.
type Config struct {
	Roots []string `yaml:"roots"`
	path  string
}

// AppConfigPath returns the canonical path to the global m2a.yml.
func AppConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "m2a", "m2a.yml"), nil
}

// Load reads the global config from the canonical location.
// A missing file is not an error — returns an empty Config.
func Load() (*Config, error) {
	path, err := AppConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads from an explicit path. Used in tests and CLI helpers.
// A missing file is not an error — returns an empty Config.
func LoadFrom(path string) (*Config, error) {
	cfg := &Config{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}

// Save writes to the path the config was loaded from.
func (c *Config) Save() error {
	return c.SaveTo(c.path)
}

// SaveTo writes to an explicit path, creating parent directories as needed.
func (c *Config) SaveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// AddRoot appends root if not already present (idempotent).
func (c *Config) AddRoot(root string) {
	for _, r := range c.Roots {
		if r == root {
			return
		}
	}
	c.Roots = append(c.Roots, root)
}

// RemoveRoot removes root if present, preserving order.
func (c *Config) RemoveRoot(root string) {
	out := c.Roots[:0]
	for _, r := range c.Roots {
		if r != root {
			out = append(out, r)
		}
	}
	c.Roots = out
}
