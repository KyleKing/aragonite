package github_test

import (
	"strings"
	"testing"

	"github.com/kyleking/aragonite/forge/github"
)

//nolint:paralleltest // asserts against the shared gh runner stub
func TestDependabotAlertsGroupsBySeverity(t *testing.T) {
	alertsJSON := []byte(`[
		{"security_advisory": {"severity": "high"}},
		{"security_advisory": {"severity": "high"}},
		{"security_advisory": {"severity": "low"}}
	]`)

	ctx, calls := stubRunGH(alertsJSON, nil)

	counts := github.DependabotAlerts(ctx, "/repo", "acme/app")
	if counts["high"] != 2 || counts["low"] != 1 {
		t.Errorf("alert counts = %v, want 2 high and 1 low", counts)
	}

	if joined := strings.Join((*calls)[0], " "); !strings.Contains(joined, "state=open") {
		t.Errorf("alerts call %q does not ask for open alerts only", joined)
	}
}

//nolint:paralleltest // asserts against the shared gh runner stub
func TestDependabotAlertsAreEmptyWhenAccessIsDenied(t *testing.T) {
	ctx, _ := stubRunGH(nil, errGHFailed)

	if counts := github.DependabotAlerts(ctx, "/repo", "acme/archived"); len(counts) != 0 {
		t.Errorf("a denied endpoint reported %v, want an empty map", counts)
	}

	if counts := github.DependabotAlerts(ctx, "/repo", ""); len(counts) != 0 {
		t.Errorf("a repo with no remote reported %v, want an empty map", counts)
	}
}
