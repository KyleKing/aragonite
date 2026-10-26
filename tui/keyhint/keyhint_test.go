package keyhint_test

import (
	"slices"
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
		{"a closing bracket", "] go to", keyhint.Hint{Key: "]", What: "go to"}},
		{"an opening bracket", "[ go back", keyhint.Hint{Key: "[", What: "go back"}},
		{"a bracket in the word", "] hunk", keyhint.Hint{Key: "]", What: "] hunk"}},
		{"a bracket off", "] go to", keyhint.Hint{Key: "]", What: "go to", Off: true}},
		{"a bracketed key off", "[x] y", keyhint.Hint{Key: "x", What: "y", Off: true}},
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

func TestFit(t *testing.T) {
	t.Parallel()

	hints := []keyhint.Hint{
		{Key: "j", What: "move", Priority: 2},
		{Key: "e", What: "edit", Priority: 1},
		{Key: "d", What: "delete", Priority: 3, Off: true},
		{Key: "q", What: "quit", Priority: 9},
	}

	tests := []struct {
		name  string
		want  string
		width int
	}{
		{"all fit, Off stays drawn", "[j] move  [e]dit  [d] delete  [q]uit", 40},
		{"Off goes before a lower priority", "[j] move  [e]dit  [q]uit", 30},
		{"then the lowest priority", "[j] move  [q]uit", 20},
		{"then the next", "[q]uit", 10},
		{"the last one is clipped", "[q]u", 4},
		{"no room draws nothing", "", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := keyhint.Fit(plain, hints, tc.width)
			if got != tc.want {
				t.Errorf("Fit(%d) = %q, want %q", tc.width, got, tc.want)
			}
		})
	}
}

func TestBareDropsTheBracketsAndKeepsTheGap(t *testing.T) {
	t.Parallel()

	bare := keyhint.Styles{Key: lipgloss.NewStyle(), Text: lipgloss.NewStyle(), Bare: true, Gap: "   "}
	hints := []keyhint.Hint{
		{Key: "enter", What: "review", Priority: 9},
		{Key: "C", What: "check out", Priority: 1},
		{Key: "s", What: "sort", Off: true},
		{Key: "?", What: "help", Priority: 9},
	}

	tests := []struct {
		name  string
		want  string
		width int
	}{
		{"all fit", "enter review   C check out   s sort   ? help", 60},
		{"the off key goes first", "enter review   C check out   ? help", 36},
		{"then the lowest priority, whole", "enter review   ? help", 30},
		{"never mid-word", "enter review   ? help", 21},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := keyhint.Fit(bare, hints, tc.width); got != tc.want {
				t.Errorf("Fit(%d) = %q, want %q", tc.width, got, tc.want)
			}
		})
	}

	if got := keyhint.One(bare, keyhint.Hint{Key: "]", What: "next", Off: true}); got != "] next" {
		t.Errorf("an off bare hint = %q, want it flat and unbracketed", got)
	}
}

func TestPanel_AlignsKeysAndCountsAndFitsTheWidth(t *testing.T) {
	t.Parallel()

	s := keyhint.PanelStyles{Frame: lipgloss.NewStyle().Border(lipgloss.NormalBorder())}
	hints := []keyhint.Hint{
		{Key: "a", What: "all", Count: "12", On: true},
		{Key: "esc", What: "a description much too long", Count: "3", Kids: []keyhint.Hint{{Key: "x"}}},
	}

	tests := []struct {
		name  string
		want  []string
		width int
	}{
		{name: "roomy", width: 60, want: []string{
			"┌─────────────────────────────────────┐",
			"│  a  all                           12│",
			"│esc  a description much too long…   3│",
			"└─────────────────────────────────────┘",
		}},
		{name: "tight", width: 20, want: nil},
		{name: "no room inside the frame", width: 2, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := keyhint.Panel(s, hints, tc.width)
			if tc.want == nil {
				for _, l := range got {
					if w := lipgloss.Width(l); w > tc.width {
						t.Errorf("row %q is %d cells wide on a %d-cell width", l, w, tc.width)
					}
				}

				return
			}

			if !slices.Equal(got, tc.want) {
				t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}
