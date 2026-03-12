package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens (or creates) the SQLite database at the given path,
// runs all pending migrations, and returns a ready-to-use *sqlx.DB.
// logger is passed to goose for migration output; pass goose.NopLogger()
// to silence it.
func Open(dbPath string, logger goose.Logger) (*sqlx.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sqlx.Open("sqlite", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite doesn't support concurrent writers.

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	if err := runMigrations(db.DB, logger); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// OpenMemory opens an in-memory SQLite database for testing.
func OpenMemory() (*sqlx.DB, error) {
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}
	if err := runMigrations(db.DB, goose.NopLogger()); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func runMigrations(db *sql.DB, logger goose.Logger) error {
	goose.SetLogger(logger)
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}
