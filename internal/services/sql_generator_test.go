package services

import "testing"

func TestParseSQLResponse(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		wantSQL         string
		wantExplanation string
	}{
		{
			name:            "code block with explanation",
			raw:             "```sql\nSELECT name FROM recipes WHERE rating >= 4\n```\nEXPLANATION: Finds highly rated recipes",
			wantSQL:         "SELECT name FROM recipes WHERE rating >= 4",
			wantExplanation: "Finds highly rated recipes",
		},
		{
			name:            "code block no explanation",
			raw:             "```sql\nSELECT * FROM recipes\n```",
			wantSQL:         "SELECT * FROM recipes",
			wantExplanation: "",
		},
		{
			name:            "bare SQL with explanation",
			raw:             "SELECT name FROM recipes\nEXPLANATION: Gets recipe names",
			wantSQL:         "SELECT name FROM recipes",
			wantExplanation: "Gets recipe names",
		},
		{
			name:            "bare SQL no explanation",
			raw:             "SELECT * FROM recipes",
			wantSQL:         "SELECT * FROM recipes",
			wantExplanation: "",
		},
		{
			name:            "multiline SQL in code block",
			raw:             "```sql\nSELECT r.name\nFROM recipes r\nJOIN recipe_tags rt ON r.id = rt.recipe_id\nWHERE rt.tag_id = 1\n```",
			wantSQL:         "SELECT r.name\nFROM recipes r\nJOIN recipe_tags rt ON r.id = rt.recipe_id\nWHERE rt.tag_id = 1",
			wantExplanation: "",
		},
		{
			name:            "plain code block without sql tag",
			raw:             "```\nSELECT 1\n```",
			wantSQL:         "SELECT 1",
			wantExplanation: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSQL, gotExplanation := parseSQLResponse(tt.raw)
			if gotSQL != tt.wantSQL {
				t.Errorf("parseSQLResponse() SQL = %q, want %q", gotSQL, tt.wantSQL)
			}
			if gotExplanation != tt.wantExplanation {
				t.Errorf("parseSQLResponse() explanation = %q, want %q", gotExplanation, tt.wantExplanation)
			}
		})
	}
}

func TestSchemaPrompt(t *testing.T) {
	prompt := schemaPrompt("sqlite", "CREATE TABLE foo (id INTEGER)")
	if prompt == "" {
		t.Fatal("schemaPrompt returned empty string")
	}
	if !contains(prompt, "SELECT") {
		t.Error("prompt should mention SELECT")
	}
	if !contains(prompt, "foo") {
		t.Error("prompt should contain the schema")
	}
	if !contains(prompt, "sqlite") {
		t.Error("prompt should mention the dialect")
	}

	pgPrompt := schemaPrompt("postgres", "CREATE TABLE bar (id BIGSERIAL)")
	if !contains(pgPrompt, "ILIKE") {
		t.Error("postgres prompt should mention ILIKE")
	}
	if !contains(pgPrompt, "COALESCE") {
		t.Error("postgres prompt should mention COALESCE")
	}

	sqPrompt := schemaPrompt("sqlite", "CREATE TABLE baz (id INTEGER)")
	if !contains(sqPrompt, "GROUP_CONCAT") {
		t.Error("sqlite prompt should mention GROUP_CONCAT")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
