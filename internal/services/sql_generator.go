package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/djcp/enplace/internal/db"
)

// schemaPrompt builds the system prompt for NL→SQL generation.
func schemaPrompt(dialect, schemaText string) string {
	var dialectRules string
	switch dialect {
	case "postgres":
		dialectRules = `Use $1, $2, etc. for parameters (though you should generate complete SQL without parameters).
- Use ILIKE for case-insensitive matching.
- Use STRING_AGG() for aggregating strings.
- Use COALESCE() instead of IFNULL().
- Use CURRENT_TIMESTAMP AT TIME ZONE 'UTC' for timestamps.
- Use ILIKE '%term%' for case-insensitive LIKE searches.`
	case "sqlite":
		dialectRules = `Use ? for parameters (though you should generate complete SQL without parameters).
- Use COLLATE NOCASE for case-insensitive matching.
- Use GROUP_CONCAT() for aggregating strings.
- Use IFNULL() or COALESCE().
- Use datetime('now') for timestamps.
- Use LIKE '%term%' with COLLATE NOCASE for case-insensitive searches.`
	default:
		dialectRules = "Use standard SQL syntax."
	}

	return fmt.Sprintf(`You are a SQL query generator for a recipe database. The user will ask a
question in natural language. Generate a single SELECT query that answers it.

DATABASE SCHEMA (%s):
%s

RULES:
- Generate ONLY SELECT queries. Never generate INSERT, UPDATE, DELETE, DROP,
  ALTER, CREATE, TRUNCATE, GRANT, or REVOKE statements.
- Use the exact table and column names from the schema above.
- The database is %s. %s
- ALWAYS include recipes.id (aliased as "id") in the SELECT clause. The
  application uses this to fetch full recipe data. Example:
  SELECT r.id, r.name FROM recipes r ...
  The "id" column must be present in every query.
- Return ONLY the SQL query, no explanation, no markdown code fences.
- Always include a LIMIT clause (default 50) unless the user explicitly asks
  for all results.
- For date comparisons, use the appropriate syntax for %s.
- If the question is ambiguous, generate the most reasonable query.
- When filtering by tag, join recipe_tags and tags, and filter by
  tags.context and tags.name. Common contexts: "courses", "cultural_influences",
  "cooking_methods", "dietary_restrictions".
- When filtering by ingredient, join recipe_ingredients and ingredients.
- The recipes.status column has values: "draft", "review", "published".
- rating is INTEGER 1-5 or NULL.
- is_bread is BOOLEAN (0/1 on SQLite, TRUE/FALSE on PostgreSQL).
- The ingredients table has an ingredient_type column with values:
  "flour", "dry", "wet", "fat", "starter", or "" (blank).`,
		dialect, schemaText, dialect, dialectRules, dialect)
}

// sqlBlockRE matches ```sql ... ``` or ``` ... ``` blocks.
var sqlBlockRE = regexp.MustCompile("(?s)```(?:sql)?\\s*(.*?)\\s*```")

// NL→SQL cache: maps question → generated SQL to avoid redundant inference.
var (
	sqlCacheMu sync.Mutex
	sqlCache   = make(map[string]string)
)

// parseSQLResponse extracts the SQL query and optional explanation from Claude's response.
func parseSQLResponse(raw string) (sql, explanation string) {
	raw = strings.TrimSpace(raw)

	// Try to extract from code block first.
	if m := sqlBlockRE.FindStringSubmatch(raw); len(m) > 1 {
		sql = strings.TrimSpace(m[1])
		// Everything outside the code block is the explanation.
		rest := strings.TrimSpace(strings.Replace(raw, m[0], "", 1))
		if strings.HasPrefix(rest, "EXPLANATION:") {
			explanation = strings.TrimSpace(strings.TrimPrefix(rest, "EXPLANATION:"))
		} else if rest != "" {
			explanation = rest
		}
		return sql, explanation
	}

	// No code block — the whole response is SQL (possibly with trailing explanation).
	lines := strings.Split(raw, "\n")
	var sqlLines []string
	var explanationLines []string
	inSQL := true
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inSQL && (strings.HasPrefix(strings.ToUpper(trimmed), "EXPLANATION:") || strings.HasPrefix(strings.ToUpper(trimmed), "EXPLANATION:")) {
			inSQL = false
			explanation = strings.TrimSpace(strings.TrimPrefix(trimmed, "EXPLANATION:"))
			explanation = strings.TrimSpace(strings.TrimPrefix(explanation, "explanation:"))
			continue
		}
		if inSQL {
			sqlLines = append(sqlLines, line)
		} else {
			explanationLines = append(explanationLines, line)
		}
	}
	sql = strings.TrimSpace(strings.Join(sqlLines, "\n"))
	if len(explanationLines) > 0 && explanation == "" {
		explanation = strings.TrimSpace(strings.Join(explanationLines, "\n"))
	}
	return sql, explanation
}

// GenerateSQLResult holds the output of NL→SQL generation.
type GenerateSQLResult struct {
	SQL         string
	Explanation string
}

// GenerateSQL takes a natural language question and the database schema,
// sends it to Claude, and returns the generated SQL + a human explanation.
func GenerateSQL(ctx context.Context, client AIClient, model, dialect, question string) (*GenerateSQLResult, error) {
	start := time.Now()

	// Check cache first.
	sqlCacheMu.Lock()
	if cached, ok := sqlCache[question]; ok {
		sqlCacheMu.Unlock()
		slog.Default().Debug("query generation cache hit",
			"question", truncateLog(question),
		)
		return &GenerateSQLResult{SQL: cached}, nil
	}
	sqlCacheMu.Unlock()

	slog.Default().Debug("query generation started",
		"dialect", dialect,
		"question", truncateLog(question),
	)

	var schemaText string
	switch dialect {
	case "postgres":
		schemaText = PostgresSchema
	default:
		schemaText = SQLiteSchema
	}

	sysPrompt := schemaPrompt(dialect, schemaText)
	raw, err := client.Complete(ctx, model, sysPrompt, question)
	if err != nil {
		slog.Default().Error("query generation failed",
			"error", err,
			"question", truncateLog(question),
		)
		return nil, fmt.Errorf("generating SQL: %w", err)
	}

	sql, explanation := parseSQLResponse(raw)
	if sql == "" {
		err := fmt.Errorf("no SQL in response: %s", raw)
		slog.Default().Error("query generation failed",
			"error", err,
			"question", truncateLog(question),
		)
		return nil, err
	}

	if err := ValidateReadOnly(sql); err != nil {
		slog.Default().Error("query generation failed",
			"error", err,
			"question", truncateLog(question),
		)
		return nil, fmt.Errorf("generated SQL is not read-only: %w", err)
	}

	slog.Default().Debug("query generation complete",
		"sql", truncateLog(sql),
		"has_explanation", explanation != "",
		"duration", time.Since(start),
	)

	// Store in cache.
	sqlCacheMu.Lock()
	sqlCache[question] = sql
	sqlCacheMu.Unlock()

	return &GenerateSQLResult{
		SQL:         sql,
		Explanation: explanation,
	}, nil
}

// ExecuteQueryResult holds the output of query execution.
type ExecuteQueryResult struct {
	Columns  []string
	Rows     [][]string
	Duration time.Duration
}

// ExecuteQuery runs a read-only SQL query against the database and returns
// column names + rows as string slices. It enforces the read-only constraint.
func ExecuteQuery(database *db.DB, query string) ([]string, [][]string, time.Duration, error) {
	if err := ValidateReadOnly(query); err != nil {
		return nil, nil, 0, fmt.Errorf("query rejected: %w", err)
	}

	start := time.Now()
	slog.Default().Debug("query execution started",
		"query", truncateLog(query),
	)

	// Use the raw *sqlx.DB to get generic rows.
	rows, err := database.Queryx(query)
	if err != nil {
		slog.Default().Error("query execution failed",
			"error", err,
			"query", truncateLog(query),
		)
		return nil, nil, 0, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		slog.Default().Error("query execution failed",
			"error", err,
			"query", truncateLog(query),
		)
		return nil, nil, 0, fmt.Errorf("getting columns: %w", err)
	}

	var result [][]string
	for rows.Next() {
		// Scan into []interface{} for maximum flexibility.
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			slog.Default().Error("query execution failed",
				"error", err,
				"query", truncateLog(query),
			)
			return nil, nil, 0, fmt.Errorf("scanning row: %w", err)
		}

		row := make([]string, len(columns))
		for i, val := range values {
			if val == nil {
				row[i] = "NULL"
			} else {
				switch v := val.(type) {
				case []byte:
					row[i] = string(v)
				case time.Time:
					row[i] = v.Format(time.RFC3339)
				case bool:
					if v {
						row[i] = "true"
					} else {
						row[i] = "false"
					}
				default:
					row[i] = fmt.Sprintf("%v", v)
				}
			}
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		slog.Default().Error("query execution failed",
			"error", err,
			"query", truncateLog(query),
		)
		return nil, nil, 0, fmt.Errorf("iterating rows: %w", err)
	}

	duration := time.Since(start)
	slog.Default().Debug("query execution complete",
		"row_count", len(result),
		"column_count", len(columns),
		"duration", duration,
	)

	return columns, result, duration, nil
}

// truncateLog truncates a string to 120 characters for log output.
func truncateLog(s string) string {
	if len(s) > 120 {
		return s[:117] + "..."
	}
	return s
}

// ScanRow is a helper to scan a single row into a destination.
func ScanRow(row *sql.Row, dest ...interface{}) error {
	return row.Scan(dest...)
}
