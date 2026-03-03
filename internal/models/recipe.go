package models

import "time"

// Status values mirror the Rails workflow.
const (
	StatusDraft            = "draft"
	StatusProcessing       = "processing"
	StatusProcessingFailed = "processing_failed"
	StatusReview           = "review"
	StatusPublished        = "published"
	StatusRejected         = "rejected"
)

// TagContext values mirror Rails acts_as_taggable_on contexts.
const (
	TagContextCookingMethods      = "cooking_methods"
	TagContextCulturalInfluences  = "cultural_influences"
	TagContextCourses             = "courses"
	TagContextDietaryRestrictions = "dietary_restrictions"
)

// AllTagContexts in display order.
var AllTagContexts = []string{
	TagContextCourses,
	TagContextCookingMethods,
	TagContextCulturalInfluences,
	TagContextDietaryRestrictions,
}

// Recipe is the core domain model.
type Recipe struct {
	ID              int64     `db:"id"`
	Name            string    `db:"name"`
	Description     string    `db:"description"`
	Directions      string    `db:"directions"`
	PreparationTime *int      `db:"preparation_time"`
	CookingTime     *int      `db:"cooking_time"`
	Servings        *int      `db:"servings"`
	ServingUnits    string    `db:"serving_units"`
	SourceURL       string    `db:"source_url"`
	SourceText      string    `db:"source_text"`
	Status          string    `db:"status"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`

	// Populated on load when needed.
	Ingredients []RecipeIngredient `db:"-"`
	Tags        []Tag              `db:"-"`
}

// IsPublished returns true when the recipe is public.
func (r *Recipe) IsPublished() bool { return r.Status == StatusPublished }

// IsProcessing returns true when AI extraction is in flight.
func (r *Recipe) IsProcessing() bool { return r.Status == StatusProcessing }

// IsFailed returns true when AI extraction failed.
func (r *Recipe) IsFailed() bool { return r.Status == StatusProcessingFailed }

// TagsByContext returns all tags matching the given context.
func (r *Recipe) TagsByContext(ctx string) []string {
	var names []string
	for _, t := range r.Tags {
		if t.Context == ctx {
			names = append(names, t.Name)
		}
	}
	return names
}

// TimingSummary returns a human-readable timing string.
func (r *Recipe) TimingSummary() string {
	var parts []string
	if r.PreparationTime != nil && *r.PreparationTime > 0 {
		parts = append(parts, formatMinutes("Prep", *r.PreparationTime))
	}
	if r.CookingTime != nil && *r.CookingTime > 0 {
		parts = append(parts, formatMinutes("Cook", *r.CookingTime))
	}
	if len(parts) == 0 {
		return ""
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "  ·  "
		}
		result += p
	}
	return result
}

func formatMinutes(label string, minutes int) string {
	if minutes < 60 {
		return label + " " + itoa(minutes) + "m"
	}
	h := minutes / 60
	m := minutes % 60
	if m == 0 {
		return label + " " + itoa(h) + "h"
	}
	return label + " " + itoa(h) + "h " + itoa(m) + "m"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
