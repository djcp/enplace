package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultModel       = "claude-haiku-4-5-20251001"
	DefaultMaxLogLines = 10_000
	ConfigDirName      = "enplace"
	ConfigFile         = "config.json"
	DBFile             = "recipes.db"
	LogFile            = "enplace.log"
)

// Config holds all persistent user settings.
type Config struct {
	AnthropicAPIKey string `json:"anthropic_api_key"`
	AnthropicModel  string `json:"anthropic_model"`
	DBPath          string `json:"db_path"`
	// Credits is displayed left-aligned in the footer of exported recipe files.
	// Use it to claim authorship (e.g. "Chef Jane Smith · myrecipeblog.com").
	Credits string `json:"credits,omitempty"`
	// MaxLogLines caps the log file size. When the file exceeds this many lines
	// on startup it is trimmed, keeping the most recent entries. 0 means use
	// DefaultMaxLogLines.
	MaxLogLines int `json:"max_log_lines,omitempty"`
	// PostgresDSN is the PostgreSQL connection string. When set, enplace uses
	// PostgreSQL instead of local SQLite.
	PostgresDSN string `json:"postgres_dsn,omitempty"`
}

// Driver returns "postgres" when PostgresDSN is set, "sqlite3" otherwise.
func (c *Config) Driver() string {
	if c.PostgresDSN != "" {
		return "postgres"
	}
	return "sqlite3"
}

// MaskDSN returns a display-safe version of a postgres DSN with credentials removed.
// URL format:  postgres://user:pass@host:5432/db?sslmode=require  →  postgres://host:5432/db
// Key=value:   host=myhost user=dan password=s3cr3t dbname=mydb   →  host=myhost dbname=mydb
// Falls back to first 30 chars + "…" if parsing fails.
func MaskDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	// Try URL format
	if u, err := url.Parse(dsn); err == nil && u.Host != "" {
		u.User = nil
		return u.String()
	}
	// Key=value format: drop user= and password= pairs
	var kept []string
	for _, part := range strings.Fields(dsn) {
		key := strings.ToLower(strings.SplitN(part, "=", 2)[0])
		if key == "user" || key == "password" {
			continue
		}
		kept = append(kept, part)
	}
	if len(kept) > 0 {
		return strings.Join(kept, " ")
	}
	// Fallback
	if len(dsn) > 30 {
		return dsn[:30] + "…"
	}
	return dsn
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
	if cfg.MaxLogLines == 0 {
		cfg.MaxLogLines = DefaultMaxLogLines
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

// LogPath returns the path to the application log file, co-located with the
// database in the XDG data directory.
func (c *Config) LogPath() (string, error) {
	return defaultLogPath()
}

func defaultDBPath() (string, error) {
	return xdgDataPath(DBFile)
}

func defaultLogPath() (string, error) {
	return xdgDataPath(LogFile)
}

func xdgDataPath(file string) (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, ConfigDirName, file), nil
}

func defaultConfig() (*Config, error) {
	dbPath, err := defaultDBPath()
	if err != nil {
		return nil, err
	}
	return &Config{
		AnthropicModel: DefaultModel,
		DBPath:         dbPath,
		MaxLogLines:    DefaultMaxLogLines,
	}, nil
}
