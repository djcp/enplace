// Package logging provides a shared slog-based logger that writes to a
// capped log file. All subsystems (migrations, pipeline, etc.) should use
// the logger returned by Open rather than writing to stderr directly.
package logging

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
)

// Open opens (or creates) the log file at logPath, trims it to at most
// maxLines lines if it has grown beyond that, and returns an *slog.Logger
// that appends to it in logfmt format.
//
// The caller is responsible for closing the returned *os.File when done.
func Open(logPath string, maxLines int) (*slog.Logger, *os.File, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return nil, nil, fmt.Errorf("creating log directory: %w", err)
	}

	if err := trimFile(logPath, maxLines); err != nil {
		return nil, nil, fmt.Errorf("trimming log file: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("opening log file: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return logger, f, nil
}

// GooseLogger returns a goose.Logger that writes via the given *slog.Logger.
func GooseLogger(logger *slog.Logger) goose.Logger {
	return &gooseAdapter{logger: logger}
}

type gooseAdapter struct {
	logger *slog.Logger
}

func (a *gooseAdapter) Fatalf(format string, v ...interface{}) {
	a.logger.Error(fmt.Sprintf(format, v...))
}

func (a *gooseAdapter) Printf(format string, v ...interface{}) {
	a.logger.Info(fmt.Sprintf(format, v...))
}

// trimFile rewrites path keeping only the last maxLines lines.
// If the file does not exist or has fewer lines than maxLines, it is left
// unchanged. maxLines <= 0 is treated as no limit.
func trimFile(path string, maxLines int) error {
	if maxLines <= 0 {
		return nil
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	lines, err := readLines(f)
	f.Close()
	if err != nil {
		return err
	}

	if len(lines) <= maxLines {
		return nil
	}

	lines = lines[len(lines)-maxLines:]

	return os.WriteFile(path, joinLines(lines), 0o600)
}

func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func joinLines(lines []string) []byte {
	total := 0
	for _, l := range lines {
		total += len(l) + 1
	}
	buf := make([]byte, 0, total)
	for _, l := range lines {
		buf = append(buf, l...)
		buf = append(buf, '\n')
	}
	return buf
}
