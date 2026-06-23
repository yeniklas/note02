package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Repo    RepoConfig    `toml:"repo"`
	Display DisplayConfig `toml:"display"`
	Journal JournalConfig `toml:"journal"`
	Archive ArchiveConfig `toml:"archive"`
}

type RepoConfig struct {
	Path string `toml:"path"`
}

type DisplayConfig struct {
	Markdown bool `toml:"markdown"`
}

type JournalConfig struct {
	Tags []string `toml:"tags"`
}

func (j JournalConfig) EffectiveTags() []string {
	if len(j.Tags) == 0 {
		return []string{"journal"}
	}
	return j.Tags
}

type ArchiveConfig struct {
	Tag string `toml:"tag"`
}

// EffectiveTag returns the tag that marks a note as archived, defaulting to
// "archived" when unset.
func (a ArchiveConfig) EffectiveTag() string {
	if a.Tag == "" {
		return "archived"
	}
	return a.Tag
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Display: DisplayConfig{Markdown: true},
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "note02", "config.toml"), nil
}
