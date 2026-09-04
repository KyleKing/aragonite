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
		want string
		hint keyhint.Hint
	}{
		{"at the start", "[p]r", keyhint.Hint{Key: "p", What: "pr"}},
		{"a word in", "post [o]ne", keyhint.Hint{Key: "o", What: "post one"}},
		{"inside a word", "s[u]bmit", keyhint.Hint{Key: "u", What: "submit"}},
		{"shifted keeps its case", "[S]ubmit", keyhint.Hint{Key: "S", What: "submit"}},
		{"not in the word", "[tab] switch", keyhint.Hint{Key: "tab", What: "switch"}},
		{"punctuation", "[/] search", keyhint.Hint{Key: "/", What: "search"}},
		{"nothing to sit in", "[q] ", keyhint.Hint{Key: "q", What: ""}},
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

// A heading sits in the description column, so the keys under it keep the one
// edge a reader scans, and it is drawn in its own face.
func TestHelpDrawsAHeadingWithTheDescriptions(t *testing.T) {
	t.Parallel()

	hints := []keyhint.Hint{
		{What: "moving", Head: true},
		{Key: "j / k", What: "move a line"},
	}

	// Measured with no face on the heading, since an escape in the line would
	// make a byte index lie about the column.
	got := keyhint.Help(plain, hints, 40)
	if len(got) != 2 {
		t.Fatalf("Help() gave %d lines, want 2: %q", len(got), got)
	}

	if head, desc := strings.Index(got[0], "moving"), strings.Index(got[1], "move a line"); head != desc {
		t.Errorf("the heading starts at column %d and the description at %d: %q", head, desc, got[0])
	}

	styled := plain
	styled.Head = lipgloss.NewStyle().Bold(true)

	if got := keyhint.Help(styled, hints, 40); !strings.Contains(got[0], styled.Head.Render("moving")) {
		t.Errorf("the heading is not drawn in Head: %q", got[0])
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
