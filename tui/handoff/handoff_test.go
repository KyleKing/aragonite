package handoff_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/tui/handoff"
)

var hostile = []string{
	"feature/plain",
	"$(touch pwned)",
	"a;rm -rf /",
	"`id`",
	"it's",
	`say "hi" \ done`,
	"two\nlines\n",
	"spaced  out",
	"*",
	"-n",
	"café/ünï",
	"${HOME}",
}

func TestExpand_QuotesHostileValues(t *testing.T) {
	t.Parallel()

	for _, value := range hostile {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			script, err := handoff.Expand("printf %s {branch}", map[string]string{"branch": value})
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}

			// #nosec G204 -- runs the expansion under test with a throwaway directory.
			cmd := exec.CommandContext(t.Context(), "sh", "-c", script)
			cmd.Dir = t.TempDir()

			got, err := cmd.Output()
			if err != nil {
				t.Fatalf("sh -c %q: %v", script, err)
			}

			if string(got) != value {
				t.Errorf("output %q, want %q", got, value)
			}
		})
	}
}

func TestExpand_Errors(t *testing.T) {
	t.Parallel()

	values := map[string]string{"path": "/repo", "pr": ""}

	tests := []struct {
		name     string
		template string
		want     error
		mention  string
	}{
		{"unknown", "tool {path} {nope}", handoff.ErrUnknownPlaceholder, "{nope}"},
		{"empty", "second-look {pr}", handoff.ErrEmptyValue, "{pr}"},
		{"unused empty is fine", "ls {path}", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := handoff.Expand(tt.template, values)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}

			if tt.want != nil && !strings.Contains(err.Error(), tt.mention) {
				t.Errorf("err %q does not name %s", err, tt.mention)
			}
		})
	}
}

func TestNeeds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		template string
		want     []string
	}{
		{"second-look {pr} --repo {repo}", []string{"pr", "repo"}},
		{"cd {path} && git switch {branch} && echo {path}", []string{"branch", "path"}},
		{"lazygit", nil},
		{"echo ${HOME} {a b}", []string{"HOME"}},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			t.Parallel()

			got := handoff.Needs(tt.template)
			if len(got) != len(tt.want) {
				t.Fatalf("Needs = %v, want %v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("Needs = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestRun_RefusesBadTemplate(t *testing.T) {
	t.Parallel()

	cmd, err := handoff.Run("review", "second-look {pr}", map[string]string{"pr": ""}, t.TempDir(), nil)
	if !errors.Is(err, handoff.ErrEmptyValue) || cmd != nil {
		t.Fatalf("Run = (%v, %v), want nil command and ErrEmptyValue", cmd, err)
	}

	cmd, err = handoff.Run("review", "true", nil, t.TempDir(), nil)
	if err != nil || cmd == nil {
		t.Fatalf("Run = (%v, %v), want a command", cmd, err)
	}
}
