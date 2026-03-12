package logging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/djcp/enplace/internal/logging"
)

func TestOpen_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	logger, f, err := logging.Open(path, 100)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer f.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("log file was not created")
	}

	_ = logger
}

func TestOpen_CreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "app.log")

	_, f, err := logging.Open(path, 100)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer f.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("log file was not created in nested directory")
	}
}

func TestOpen_TrimsToMaxLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Write 20 lines of existing content.
	var sb strings.Builder
	for i := 1; i <= 20; i++ {
		sb.WriteString("line\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	_, f, err := logging.Open(path, 10)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Count(string(data), "\n")
	if got != 10 {
		t.Errorf("expected 10 lines after trim, got %d", got)
	}
}

func TestOpen_DoesNotTrimWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, f, err := logging.Open(path, 100)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("expected content unchanged, got %q", string(data))
	}
}

func TestOpen_AppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	if err := os.WriteFile(path, []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	logger, f, err := logging.Open(path, 1000)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	logger.Info("new entry")
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "existing\n") {
		t.Error("existing content was not preserved")
	}
	if !strings.Contains(string(data), "new entry") {
		t.Error("new log entry was not appended")
	}
}

func TestGooseLogger_WritesToLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	logger, f, err := logging.Open(path, 1000)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	gl := logging.GooseLogger(logger)
	gl.Printf("migration applied: %s", "001_initial.sql")
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "migration applied: 001_initial.sql") {
		t.Errorf("expected log entry in file, got: %s", string(data))
	}
}
