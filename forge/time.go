package forge

import (
	"fmt"
	"time"
)

const (
	emDash = "—"

	hoursPerDay  = 24
	daysPerWeek  = 7
	daysPerMonth = 30
	daysPerYear  = 365
)

// RelativeTime renders t as a human-readable duration relative to now (e.g. "3 days ago").
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return emDash
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 min ago"
		}

		return fmt.Sprintf("%d mins ago", mins)
	case diff < hoursPerDay*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}

		return fmt.Sprintf("%d hours ago", hours)
	case diff < daysPerWeek*hoursPerDay*time.Hour:
		days := int(diff.Hours() / hoursPerDay)
		if days == 1 {
			return "1 day ago"
		}

		return fmt.Sprintf("%d days ago", days)
	case diff < daysPerMonth*hoursPerDay*time.Hour:
		weeks := int(diff.Hours() / hoursPerDay / daysPerWeek)
		if weeks == 1 {
			return "1 week ago"
		}

		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < daysPerYear*hoursPerDay*time.Hour:
		months := int(diff.Hours() / hoursPerDay / daysPerMonth)
		if months == 1 {
			return "1 month ago"
		}

		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / hoursPerDay / daysPerYear)
		if years == 1 {
			return "1 year ago"
		}

		return fmt.Sprintf("%d years ago", years)
	}
}
