package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultModel  = "claude-haiku-4-5-20251001"
	ConfigDirName = "gorecipes"
	ConfigFile    = "config.json"
	DBFile        = "recipes.db"
)

// Config holds all persistent user settings.
type Config struct {
	AnthropicAPIKey string `json:"anthropic_api_key"`
	AnthropicModel  string `json:"anthropic_model"`
	DBPath          string `json:"db_path"`
}

// Load reads config from the XDG config directory.
// Returns a default config (not saved) if the file doesn't exist.
func Load() (*Config, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.AnthropicModel == "" {
		cfg.AnthropicModel = DefaultModel
	}
	if cfg.DBPath == "" {
		cfg.DBPath, err = defaultDBPath()
		if err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// Save writes config to disk, creating the directory if needed.
func (c *Config) Save() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// IsConfigured returns true if an API key is set.
func (c *Config) IsConfigured() bool {
	return c.AnthropicAPIKey != ""
}

// FilePath returns the path to the config file.
func FilePath() (string, error) {
	return configFilePath()
}

func configFilePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFile), nil
}

func configDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, ConfigDirName), nil
}

func defaultDBPath() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, ConfigDirName, DBFile), nil
}

func defaultConfig() (*Config, error) {
	dbPath, err := defaultDBPath()
	if err != nil {
		return nil, err
	}
	return &Config{
		AnthropicModel: DefaultModel,
		DBPath:         dbPath,
	}, nil
}
