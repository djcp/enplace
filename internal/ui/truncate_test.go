package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		// Negative and zero max must never panic.
		{"negative max", "hello", -10, ""},
		{"zero max", "hello", 0, ""},

		// Strings that fit.
		{"exact fit", "hello", 5, "hello"},
		{"shorter than max", "hi", 10, "hi"},
		{"empty string", "", 5, ""},

		// Truncation with ellipsis.
		{"long string", "hello world", 8, "hello w…"},

		// Small max values (≤3 — no room for ellipsis).
		{"max 1", "hello", 1, "h"},
		{"max 2", "hello", 2, "he"},
		{"max 3", "hello", 3, "hel"},

		// Unicode.
		{"unicode exact", "héllo", 5, "héllo"},
		{"unicode truncate", "héllo world", 8, "héllo w…"},
		{"unicode negative", "héllo", -1, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.s, tc.max)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q; want %q", tc.s, tc.max, got, tc.want)
			}
		})
	}
}

func TestTruncateW(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		maxCols int
		want    string
	}{
		// Edge cases.
		{"negative max", "hello", -1, ""},
		{"zero max", "hello", 0, ""},
		{"empty string", "", 5, ""},

		// Strings that fit within limit.
		{"exact fit ascii", "hello", 5, "hello"},
		{"shorter than max", "hi", 10, "hi"},

		// ASCII truncation (same behaviour as truncate).
		{"ascii truncate", "hello world", 8, "hello w…"},

		// maxCols ≤ 2 — no ellipsis.
		{"max 1 ascii", "hello", 1, "h"},
		{"max 2 ascii", "hello", 2, "he"},

		// Wide-character cases.
		// 🍞 = 2 display cols, 📝 = 2 display cols.

		// 🍞 exactly fills 2 cols — no truncation needed.
		{"wide char exact 2", "🍞", 2, "🍞"},

		// "🍞 Bread" = 2+1+5 = 8 display cols; truncate to 5.
		// target = 4: 🍞(2) + ' '(1) + 'B'(1) = 4, next 'r' would exceed → "🍞 B…"
		{"wide char prefix truncate", "🍞 Bread", 5, "🍞 B…"},

		// "abc🍞xyz" = 1+1+1+2+1+1+1 = 8 cols; truncate to 6.
		// target = 5: a(1)+b(1)+c(1)+🍞(2)=5, next 'x' exceeds → "abc🍞…"
		{"wide char in middle", "abc🍞xyz", 6, "abc🍞…"},

		// Wide char at boundary: "ab🍞cd" = 1+1+2+1+1 = 6 cols; truncate to 4.
		// target = 3: a(1)+b(1) = 2 ≤ 3, then 🍞(2): 2+2 = 4 > 3 → stop before 🍞 → "ab…"
		{"wide char skipped at boundary", "ab🍞cd", 4, "ab…"},

		// Notes emoji: "Name 📝" = 4+1+2 = 7 cols; truncate to 5.
		// target = 4: N(1)+a(1)+m(1)+e(1) = 4, next ' ' would make 5 > 4 → "Name…"
		{"notes emoji suffix truncate", "Name 📝", 5, "Name…"},

		// Wide char result has the correct display width.
		{"display width correct", "🍞 abc", 4, "🍞 …"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateW(tc.s, tc.maxCols)
			if got != tc.want {
				t.Errorf("truncateW(%q, %d) = %q; want %q", tc.s, tc.maxCols, got, tc.want)
			}
			// Result must never exceed maxCols display columns (only meaningful when maxCols >= 0).
			if tc.maxCols >= 0 {
				if w := lipgloss.Width(got); w > tc.maxCols {
					t.Errorf("result display width %d exceeds maxCols %d", w, tc.maxCols)
				}
			}
		})
	}
}
