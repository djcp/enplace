package services

import "testing"

func TestValidateReadOnly(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{"simple select", "SELECT * FROM recipes", false},
		{"select with where", "SELECT name, rating FROM recipes WHERE rating >= 4", false},
		{"select with join", "SELECT r.name, t.name FROM recipes r JOIN recipe_tags rt ON r.id = rt.recipe_id", false},
		{"with cte", "WITH top AS (SELECT * FROM recipes ORDER BY rating DESC LIMIT 5) SELECT * FROM top", false},
		{"explain", "EXPLAIN SELECT * FROM recipes", false},
		{"pragma", "PRAGMA table_info(recipes)", false},
		{"select with leading whitespace", "  \n  SELECT * FROM recipes", false},
		{"select with line comment", "-- fetch all\nSELECT * FROM recipes", false},
		{"select with block comment", "/* get everything */ SELECT * FROM recipes", false},
		{"select with multiple comments", "-- first\n/* second */\nSELECT * FROM recipes", false},
		{"select after comment and semicolon", "-- comment\nSELECT 1; SELECT 2", false},

		{"insert", "INSERT INTO recipes (name) VALUES ('test')", true},
		{"update", "UPDATE recipes SET name = 'test' WHERE id = 1", true},
		{"delete", "DELETE FROM recipes WHERE id = 1", true},
		{"drop", "DROP TABLE recipes", true},
		{"alter", "ALTER TABLE recipes ADD COLUMN foo TEXT", true},
		{"create", "CREATE TABLE foo (id INTEGER)", true},
		{"truncate", "TRUNCATE TABLE recipes", true},
		{"vacuum", "VACUUM", true},
		{"reindex", "REINDEX", true},
		{"grant", "GRANT SELECT ON recipes TO user", true},

		{"injection after select", "SELECT * FROM recipes; DROP TABLE recipes", true},
		{"select then delete", "SELECT 1; DELETE FROM recipes", true},
		{"select with leading comment then drop", "-- ok\nSELECT 1;\n-- not ok\nDROP TABLE recipes", true},

		{"empty string", "", false},
		{"only semicolons", ";;;  ", false},
		{"only comments", "-- just a comment", false},
		{"only block comment", "/* nothing here */", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReadOnly(tt.sql)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateReadOnly(%q) error = %v, wantErr %v", tt.sql, err, tt.wantErr)
			}
		})
	}
}

func TestFirstKeyword(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SELECT * FROM foo", "select"},
		{"  SELECT 1", "select"},
		{"-- comment\nSELECT 1", "select"},
		{"/* block */ SELECT 1", "select"},
		{"\n\nSELECT 1", "select"},
		{"", ""},
		{"   ", ""},
		{"-- only comment", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := firstKeyword(tt.input)
			if got != tt.want {
				t.Errorf("firstKeyword(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
