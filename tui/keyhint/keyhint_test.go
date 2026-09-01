package keyhint_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/keyhint"
)

// plain styles keep the test about the placement of the brackets rather than
// about the escapes lipgloss wraps them in.
var plain = keyhint.Styles{Key: lipgloss.NewStyle(), Text: lipgloss.NewStyle()}

func TestOne(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hint keyhint.Hint
		want string
	}{
		{"at the start", keyhint.Hint{Key: "p", What: "pr"}, "[p]r"},
		{"a word in", keyhint.Hint{Key: "o", What: "post one"}, "post [o]ne"},
		{"inside a word", keyhint.Hint{Key: "u", What: "submit"}, "s[u]bmit"},
		{"shifted keeps its case", keyhint.Hint{Key: "S", What: "submit"}, "[S]ubmit"},
		{"not in the word", keyhint.Hint{Key: "tab", What: "switch"}, "[tab] switch"},
		{"punctuation", keyhint.Hint{Key: "/", What: "search"}, "[/] search"},
		{"nothing to sit in", keyhint.Hint{Key: "q", What: ""}, "[q] "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := keyhint.One(plain, tc.hint); got != tc.want {
				t.Errorf("One() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLine(t *testing.T) {
	t.Parallel()

	got := keyhint.Line(plain, []keyhint.Hint{
		{Key: "e", What: "edit"}, {Key: "tab", What: "switch"},
	})

	if want := "[e]dit" + keyhint.Gap + "[tab] switch"; got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
}

func TestHelp(t *testing.T) {
	t.Parallel()

	got := keyhint.Help(plain, []keyhint.Hint{
		{Key: "j / k", What: "move a line"},
		{Key: "S then S / a / r / c", What: "submit"},
		{What: "The keys above are the whole of it."},
	}, 60)

	want := []string{
		"                 j / k  move a line",
		"  S then S / a / r / c  submit",
		"                        The keys above are the whole of it.",
	}

	if len(got) != len(want) {
		t.Fatalf("Help() gave %d lines, want %d: %q", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A legend has to fit the frame it is drawn in, since a help screen that wraps
// is the one screen a reader cannot fall back on.
func TestHelpFitsItsWidth(t *testing.T) {
	t.Parallel()

	for _, width := range []int{40, 60, 80, 120} {
		for _, line := range keyhint.Help(plain, []keyhint.Hint{
			{Key: "S then S / a / r / c", What: strings.Repeat("long ", 40)},
			{What: strings.Repeat("prose ", 40)},
		}, width) {
			if lipgloss.Width(line) > width {
				t.Errorf("at %d columns a line is %d wide: %q", width, lipgloss.Width(line), line)
			}
		}
	}
}
