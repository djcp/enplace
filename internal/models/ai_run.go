package models

import "time"

// AIClassifierRun is an audit record for every AI pipeline call,
// mirroring the Rails ai_classifier_runs table.
type AIClassifierRun struct {
	ID           int64      `db:"id"`
	RecipeID     *int64     `db:"recipe_id"`
	ServiceClass string     `db:"service_class"`
	Adapter      string     `db:"adapter"`
	AIModel      string     `db:"ai_model"`
	SystemPrompt string     `db:"system_prompt"`
	UserPrompt   string     `db:"user_prompt"`
	RawResponse  string     `db:"raw_response"`
	Success      bool       `db:"success"`
	ErrorClass   string     `db:"error_class"`
	ErrorMessage string     `db:"error_message"`
	StartedAt    *time.Time `db:"started_at"`
	CompletedAt  *time.Time `db:"completed_at"`
	CreatedAt    time.Time  `db:"created_at"`
}

// DurationMS returns the duration in milliseconds, or -1 if not complete.
func (r *AIClassifierRun) DurationMS() int64 {
	if r.StartedAt == nil || r.CompletedAt == nil {
		return -1
	}
	return r.CompletedAt.Sub(*r.StartedAt).Milliseconds()
}
