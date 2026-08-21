package forge_test

import (
	"testing"

	"github.com/kyleking/aragonite/forge"
)

func TestPRInfoMatchesUpstream(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		owner    string
		upstream string
		pr       forge.PullRequest
		expected bool
	}{
		{
			name:     "matches regardless of local branch name",
			pr:       forge.PullRequest{HeadRef: "feature-x"},
			owner:    "kyleking",
			upstream: "origin/feature-x",
			expected: true,
		},
		{
			name:     "different head ref does not match",
			pr:       forge.PullRequest{HeadRef: "feature-x"},
			owner:    "kyleking",
			upstream: "origin/feature-y",
			expected: false,
		},
		{
			name:     "fork pr never matches on upstream alone",
			pr:       forge.PullRequest{HeadRef: "feature-x", HeadRepoOwner: "someone-else"},
			owner:    "kyleking",
			upstream: "origin/feature-x",
			expected: false,
		},
		{
			name:     "no upstream never matches",
			pr:       forge.PullRequest{HeadRef: "feature-x"},
			owner:    "kyleking",
			upstream: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.pr.MatchesUpstream(tt.owner, tt.upstream); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}
