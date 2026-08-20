package services

import (
	"fmt"
	"strings"
)

// readOnlyPrefixes are SQL statement prefixes that are safe (read-only).
var readOnlyPrefixes = []string{"select", "with", "explain", "pragma"}

// ValidateReadOnly returns an error if any statement in the SQL is a mutation.
// It splits on semicolons, strips comments, and checks each statement's leading keyword.
func ValidateReadOnly(sql string) error {
	statements := splitStatements(sql)
	for _, stmt := range statements {
		keyword := firstKeyword(stmt)
		if keyword == "" {
			continue
		}
		isReadOnly := false
		for _, prefix := range readOnlyPrefixes {
			if keyword == prefix {
				isReadOnly = true
				break
			}
		}
		if !isReadOnly {
			return fmt.Errorf("not a read-only query: %s statement not allowed", strings.ToUpper(keyword))
		}
	}
	return nil
}

// splitStatements splits SQL on semicolons, trimming whitespace and discarding empty parts.
func splitStatements(sql string) []string {
	var parts []string
	for _, s := range strings.Split(sql, ";") {
		s = strings.TrimSpace(s)
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

// firstKeyword extracts the first SQL keyword from a statement, stripping
// leading comments (-- and /* */) and whitespace.
func firstKeyword(stmt string) string {
	stmt = strings.TrimSpace(stmt)
	// Strip line comments.
	for strings.HasPrefix(stmt, "--") {
		idx := strings.Index(stmt, "\n")
		if idx < 0 {
			stmt = ""
			break
		}
		stmt = strings.TrimSpace(stmt[idx+1:])
	}
	// Strip block comments.
	for strings.HasPrefix(stmt, "/*") {
		idx := strings.Index(stmt, "*/")
		if idx < 0 {
			stmt = ""
			break
		}
		stmt = strings.TrimSpace(stmt[idx+2:])
		// There may be more comments.
	}
	stmt = strings.TrimSpace(stmt)
	if stmt == "" {
		return ""
	}
	fields := strings.Fields(stmt)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(fields[0])
}
