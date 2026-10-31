// Package config handles the optional on-disk settings file.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Config is what the user may override. Every field has a sane zero value.
type Config struct {
	City     string   `json:"city,omitempty"`
	Units    string   `json:"units,omitempty"`    // "c" or "f"
	Limit    int      `json:"limit,omitempty"`    // headlines per section
	Sections []string `json:"sections,omitempty"` // weather,world,tech,fact
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		Units:    "", // auto: follow the reported place
		Limit:    5,
		Sections: []string{"weather", "local", "world", "tech", "fact"},
	}
}

// Dir is the per-user config directory for morning.
func Dir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ".morning"
	}
	return filepath.Join(d, "morning")
}

// DefaultPath is where Load looks when no path is given.
func DefaultPath() string { return filepath.Join(Dir(), "config.json") }

// Load reads path. A missing file is not an error — defaults are returned.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultPath()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Default(), err
	}
	cfg.normalize()
	return cfg, nil
}

// Save writes the config, creating the directory if needed.
func (c Config) Save(path string) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func (c *Config) normalize() {
	d := Default()
	if c.Units != "c" && c.Units != "f" {
		c.Units = "" // anything else (including "auto") follows the place
	}
	if c.Limit <= 0 {
		c.Limit = d.Limit
	}
	if c.Limit > 20 {
		c.Limit = 20
	}
	if len(c.Sections) == 0 {
		c.Sections = d.Sections
	}
}
