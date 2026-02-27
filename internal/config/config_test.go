package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/djcp/gorecipes/internal/config"
)

func TestLoad_DefaultsWhenMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.AnthropicModel != config.DefaultModel {
		t.Errorf("expected default model %q, got %q", config.DefaultModel, cfg.AnthropicModel)
	}
	if cfg.AnthropicAPIKey != "" {
		t.Errorf("expected empty API key, got %q", cfg.AnthropicAPIKey)
	}
	if cfg.DBPath == "" {
		t.Error("expected non-empty default DB path")
	}
	if cfg.IsConfigured() {
		t.Error("expected IsConfigured() to be false with no API key")
	}
}

func TestSaveAndLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	want := &config.Config{
		AnthropicAPIKey: "sk-ant-testkey1234",
		AnthropicModel:  "claude-haiku-4-5-20251001",
		DBPath:          "/tmp/test.db",
	}

	if err := want.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}

	if got.AnthropicAPIKey != want.AnthropicAPIKey {
		t.Errorf("API key: got %q, want %q", got.AnthropicAPIKey, want.AnthropicAPIKey)
	}
	if got.AnthropicModel != want.AnthropicModel {
		t.Errorf("model: got %q, want %q", got.AnthropicModel, want.AnthropicModel)
	}
	if got.DBPath != want.DBPath {
		t.Errorf("DB path: got %q, want %q", got.DBPath, want.DBPath)
	}
}

func TestIsConfigured(t *testing.T) {
	t.Run("false when empty", func(t *testing.T) {
		cfg := &config.Config{}
		if cfg.IsConfigured() {
			t.Error("expected false")
		}
	})
	t.Run("true when key set", func(t *testing.T) {
		cfg := &config.Config{AnthropicAPIKey: "sk-ant-xxx"}
		if !cfg.IsConfigured() {
			t.Error("expected true")
		}
	})
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "nested", "deep"))

	cfg := &config.Config{
		AnthropicAPIKey: "sk-ant-test",
		AnthropicModel:  config.DefaultModel,
		DBPath:          "/tmp/r.db",
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() should create directories: %v", err)
	}

	path, err := config.FilePath()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestLoad_SetsDefaultModelWhenMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	// Write config without model field.
	cfgDir := filepath.Join(dir, "gorecipes")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"anthropic_api_key": "sk-ant-test", "db_path": "/tmp/r.db"}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AnthropicModel != config.DefaultModel {
		t.Errorf("expected default model filled in, got %q", cfg.AnthropicModel)
	}
}
