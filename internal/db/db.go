package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/djcp/enplace/internal/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrationsFS embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrationsFS embed.FS

// DB wraps *sqlx.DB and automatically rebinds ? placeholders to the
// driver's native syntax (? for SQLite, $1/$2/… for PostgreSQL).
type DB struct {
	*sqlx.DB
	driver string
}

// Driver returns "postgres" or "sqlite3".
func (d *DB) Driver() string { return d.driver }

// Get rebinds the query then delegates to sqlx.DB.Get.
func (d *DB) Get(dest interface{}, query string, args ...interface{}) error {
	return d.DB.Get(dest, d.DB.Rebind(query), args...)
}

// Select rebinds the query then delegates to sqlx.DB.Select.
func (d *DB) Select(dest interface{}, query string, args ...interface{}) error {
	return d.DB.Select(dest, d.DB.Rebind(query), args...)
}

// Exec rebinds the query then delegates to sqlx.DB.Exec.
func (d *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return d.DB.Exec(d.DB.Rebind(query), args...)
}

// QueryRow rebinds the query then delegates to sqlx.DB.QueryRow.
func (d *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return d.DB.QueryRow(d.DB.Rebind(query), args...)
}

// insertReturningID runs an INSERT and returns the new row's id.
// For PostgreSQL it appends RETURNING id and scans the result.
// For SQLite it uses LastInsertId().
func (d *DB) insertReturningID(query string, args ...interface{}) (int64, error) {
	if d.driver == "postgres" {
		var id int64
		err := d.DB.QueryRow(d.DB.Rebind(query+" RETURNING id"), args...).Scan(&id)
		return id, err
	}
	res, err := d.DB.Exec(d.DB.Rebind(query), args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// onConflictDoNothing wraps an INSERT statement to make it idempotent.
// SQLite:   INSERT INTO → INSERT OR IGNORE INTO
// Postgres: appends ON CONFLICT DO NOTHING
func (d *DB) onConflictDoNothing(query string) string {
	if d.driver == "postgres" {
		return query + " ON CONFLICT DO NOTHING"
	}
	// Replace first occurrence of "INSERT INTO" with "INSERT OR IGNORE INTO"
	const old = "INSERT INTO"
	if idx := strings.Index(strings.ToUpper(query), old); idx >= 0 {
		return query[:idx] + "INSERT OR IGNORE INTO" + query[idx+len(old):]
	}
	return query
}

// Open opens the database specified by cfg, runs all pending migrations,
// and returns a ready-to-use *DB. For SQLite it creates the directory if
// needed. For PostgreSQL it connects using cfg.PostgresDSN.
//
// Returns an error if the connection or migrations fail. PostgreSQL
// connection failures are fatal — they are not silently downgraded to SQLite.
func Open(cfg *config.Config, logger goose.Logger) (*DB, error) {
	if cfg.Driver() == "postgres" {
		return openPostgres(cfg.PostgresDSN, logger)
	}
	return openSQLite(cfg.DBPath, logger)
}

// OpenSQLite opens (or creates) a SQLite database at dbPath.
func OpenSQLite(dbPath string, logger goose.Logger) (*DB, error) {
	return openSQLite(dbPath, logger)
}

func openSQLite(dbPath string, logger goose.Logger) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}
	raw, err := sqlx.Open("sqlite", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	raw.SetMaxOpenConns(1)
	if _, err := raw.Exec("PRAGMA foreign_keys = ON"); err != nil {
		raw.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}
	if err := runMigrations(raw.DB, "sqlite3", "migrations/sqlite", sqliteMigrationsFS, logger); err != nil {
		raw.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return &DB{DB: raw, driver: "sqlite3"}, nil
}

func openPostgres(dsn string, logger goose.Logger) (*DB, error) {
	raw, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres connection: %w", err)
	}
	if err := raw.Ping(); err != nil {
		raw.Close()
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}
	if err := runMigrations(raw.DB, "postgres", "migrations/postgres", postgresMigrationsFS, logger); err != nil {
		raw.Close()
		return nil, fmt.Errorf("running postgres migrations: %w", err)
	}
	return &DB{DB: raw, driver: "postgres"}, nil
}

// OpenMemory opens an in-memory SQLite database for testing.
func OpenMemory() (*DB, error) {
	raw, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(1)
	if _, err := raw.Exec("PRAGMA foreign_keys = ON"); err != nil {
		raw.Close()
		return nil, err
	}
	if err := runMigrations(raw.DB, "sqlite3", "migrations/sqlite", sqliteMigrationsFS, goose.NopLogger()); err != nil {
		raw.Close()
		return nil, err
	}
	return &DB{DB: raw, driver: "sqlite3"}, nil
}

// TestPostgresConnection attempts to open and ping a postgres connection.
// Used for config-time validation. The connection is closed immediately.
func TestPostgresConnection(dsn string) error {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}

// SQLiteHasRecipes reports whether the SQLite DB at dbPath exists and
// contains at least one recipe. Returns (0, nil) when the file does not exist.
func SQLiteHasRecipes(dbPath string) (int, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return 0, nil
	}
	raw, err := sqlx.Open("sqlite", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return 0, err
	}
	defer raw.Close()
	raw.SetMaxOpenConns(1)
	var count int
	if err := raw.Get(&count, "SELECT COUNT(*) FROM recipes"); err != nil {
		return 0, err
	}
	return count, nil
}

func runMigrations(db *sql.DB, dialect, dir string, fs embed.FS, logger goose.Logger) error {
	goose.SetLogger(logger)
	goose.SetBaseFS(fs)
	if err := goose.SetDialect(dialect); err != nil {
		return err
	}
	return goose.Up(db, dir)
}
