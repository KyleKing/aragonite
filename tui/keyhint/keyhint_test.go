package keyhint_test

import (
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
