package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/djcp/gorecipes/internal/models"
)

// RenderRecipeDetail returns a fully formatted recipe display string.
func RenderRecipeDetail(r *models.Recipe, termWidth int) string {
	if termWidth < 40 {
		termWidth = 80
	}
	contentWidth := min(termWidth-4, 100)

	var sb strings.Builder

	// Header.
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Width(contentWidth).
		Render(r.Name))
	sb.WriteString("\n")

	// Timing & servings line.
	meta := []string{}
	if t := r.TimingSummary(); t != "" {
		meta = append(meta, MutedStyle.Render(t))
	}
	if r.Servings != nil && *r.Servings > 0 {
		units := r.ServingUnits
		if units == "" {
			units = "servings"
		}
		meta = append(meta, MutedStyle.Render(fmt.Sprintf("Serves %d %s", *r.Servings, units)))
	}
	if len(meta) > 0 {
		sb.WriteString(strings.Join(meta, MutedStyle.Render("  ·  ")))
		sb.WriteString("\n")
	}

	// Tags row.
	if tags := renderTags(r); tags != "" {
		sb.WriteString(tags)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Description.
	if r.Description != "" {
		sb.WriteString(lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#5C4A3C")).
			Width(contentWidth).
			Render(r.Description))
		sb.WriteString("\n\n")
	}

	// Ingredients.
	if len(r.Ingredients) > 0 {
		sb.WriteString(SectionLabelStyle.Render("Ingredients"))
		sb.WriteString("\n")
		sb.WriteString(renderIngredients(r.Ingredients, contentWidth))
		sb.WriteString("\n")
	}

	// Directions.
	if r.Directions != "" {
		sb.WriteString(SectionLabelStyle.Render("Directions"))
		sb.WriteString("\n")
		sb.WriteString(renderMarkdown(r.Directions, contentWidth))
	}

	// Source.
	if r.SourceURL != "" {
		sb.WriteString("\n")
		sb.WriteString(MutedStyle.Render("Source: " + r.SourceURL))
		sb.WriteString("\n")
	}

	return sb.String()
}

func renderTags(r *models.Recipe) string {
	var pills []string
	for _, ctx := range models.AllTagContexts {
		for _, name := range r.TagsByContext(ctx) {
			pills = append(pills, TagStyle(ctx).Render(name))
		}
	}
	return strings.Join(pills, "")
}

func renderIngredients(ings []models.RecipeIngredient, _ int) string {
	var sb strings.Builder
	currentSection := ""

	for _, ing := range ings {
		// Print section header if changed.
		if ing.Section != currentSection && ing.Section != "" {
			if currentSection != "" {
				sb.WriteString("\n")
			}
			sb.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorMuted).
				Render("  " + ing.Section))
			sb.WriteString("\n")
			currentSection = ing.Section
		}

		line := "  · " + ing.DisplayString()
		sb.WriteString(MutedStyle.Render("  · ") + ing.DisplayString())
		_ = line
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderMarkdown(text string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
