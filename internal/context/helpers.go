package context

import "time"

// CurrentStateBranch returns git branch or empty.
func (c *ProjectContext) CurrentStateBranch() string {
	if c.CurrentState == nil {
		return ""
	}
	return c.CurrentState.GitBranch
}

// Staleness buckets: fresh < 1 day, stale < 7 days, expired beyond.
// AI agents should prefer fresh context and warn on expired sections.
func (c *ProjectContext) Staleness(now time.Time) string {
	if c == nil || c.ExtractedAt.IsZero() {
		return "expired"
	}
	switch d := now.Sub(c.ExtractedAt); {
	case d < 24*time.Hour:
		return "fresh"
	case d < 7*24*time.Hour:
		return "stale"
	default:
		return "expired"
	}
}
