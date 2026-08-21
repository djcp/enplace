package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/djcp/enplace/internal/services"
	"github.com/djcp/enplace/internal/ui"
	"github.com/spf13/cobra"
)

var (
	queryRawSQL string
	queryJSON   bool
)

var queryCmd = &cobra.Command{
	Use:   "query [question]",
	Short: "Query your recipe database using natural language or raw SQL",
	Long: `Query your recipe database using natural language or raw SQL.

Without arguments, opens an interactive prompt. With an argument, generates
SQL from natural language and executes it.

Examples:
  enplace query "what are my highest rated Italian recipes?"
  enplace query --sql "SELECT name, rating FROM recipes WHERE rating >= 4"
  enplace query --json "show me all bread recipes"
  enplace query   (interactive prompt)`,
	Args:         cobra.MaximumNArgs(1),
	RunE:         runQuery,
	SilenceUsage: true,
}

func init() {
	queryCmd.Flags().StringVar(&queryRawSQL, "sql", "", "Execute raw SQL directly (skips AI generation)")
	queryCmd.Flags().BoolVar(&queryJSON, "json", false, "Output results as JSON")
	Root.AddCommand(queryCmd)
}

func runQuery(_ *cobra.Command, args []string) error {
	if cfg.AnthropicAPIKey == "" {
		return fmt.Errorf("query requires an Anthropic API key — run `enplace config` to set one")
	}

	dialect := "sqlite"
	if cfg.Driver() == "postgres" {
		dialect = "postgres"
	}

	// Raw SQL mode: --sql flag.
	if queryRawSQL != "" {
		return executeQuery(queryRawSQL, dialect)
	}

	// Argument mode: question provided on command line.
	if len(args) > 0 {
		question := strings.TrimSpace(args[0])
		return generateAndExecute(question, dialect)
	}

	// Interactive mode: prompt for input.
	return interactiveQuery(dialect)
}

func interactiveQuery(dialect string) error {
	var question string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("What would you like to know?").
				Description("Ask a question about your recipes in plain English.").
				Value(&question),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	question = strings.TrimSpace(question)
	if question == "" {
		return nil
	}
	return generateAndExecute(question, dialect)
}

func generateAndExecute(question, dialect string) error {
	fmt.Printf("\n  Generating SQL from: %s\n\n", question)

	client := services.NewAnthropicClient(cfg.AnthropicAPIKey)
	ctx := context.Background()

	result, err := services.GenerateSQL(ctx, client, cfg.AnthropicModel, dialect, question)
	if err != nil {
		return fmt.Errorf("generating SQL: %w", err)
	}

	fmt.Println("  Generated SQL:")
	fmt.Printf("  %s\n\n", ui.MutedStyle.Render(indentSQL(result.SQL, "  ")))

	if result.Explanation != "" {
		fmt.Printf("  %s\n\n", ui.MutedStyle.Render(result.Explanation))
	}

	// Confirm execution.
	var confirm bool
	form2 := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[bool]().
				Title("Execute this query?").
				Options(
					huh.NewOption("Yes", true),
					huh.NewOption("No", false),
				).
				Value(&confirm),
		),
	)
	if err := form2.Run(); err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	return executeQuery(result.SQL, dialect)
}

func executeQuery(query, dialect string) error {
	if err := services.ValidateReadOnly(query); err != nil {
		return fmt.Errorf("query rejected: %w", err)
	}

	start := time.Now()
	columns, rows, _, err := services.ExecuteQuery(sqlDB, query)
	if err != nil {
		return fmt.Errorf("executing query: %w", err)
	}
	elapsed := time.Since(start)

	if queryJSON {
		return printJSON(columns, rows)
	}

	printTable(columns, rows)
	fmt.Printf("\n  %s\n\n", ui.MutedStyle.Render(fmt.Sprintf("%d row(s) (%s)", len(rows), elapsed.Round(time.Microsecond))))
	return nil
}

func printTable(columns []string, rows [][]string) {
	if len(columns) == 0 {
		return
	}

	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Header.
	fmt.Print("\n  ")
	for i, col := range columns {
		fmt.Printf("%-*s  ", widths[i], col)
	}
	fmt.Println()

	// Separator.
	fmt.Print("  ")
	for _, w := range widths {
		fmt.Printf("%s  ", strings.Repeat("-", w))
	}
	fmt.Println()

	// Rows.
	for _, row := range rows {
		fmt.Print("  ")
		for i, cell := range row {
			if i < len(widths) {
				fmt.Printf("%-*s  ", widths[i], cell)
			}
		}
		fmt.Println()
	}
}

func printJSON(columns []string, rows [][]string) error {
	var results []map[string]string
	for _, row := range rows {
		m := make(map[string]string)
		for i, col := range columns {
			if i < len(row) {
				m[col] = row[i]
			}
		}
		results = append(results, m)
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func indentSQL(sql, prefix string) string {
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		if i > 0 {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// PrintQueryResult displays query results in a formatted table.
// Exported for use by the TUI.
func PrintQueryResult(columns []string, rows [][]string, duration time.Duration) {
	printTable(columns, rows)
	fmt.Printf("\n  %s\n\n", ui.MutedStyle.Render(fmt.Sprintf("%d row(s) (%s)", len(rows), duration.Round(time.Microsecond))))
}
