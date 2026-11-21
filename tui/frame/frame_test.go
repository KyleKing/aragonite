package frame_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

func sample() frame.Frame {
	return frame.Frame{
		Title:  "gh-repo-dashboard",
		Facts:  []frame.Fact{{Text: "42 repos"}, {Text: "3 behind", Tone: frame.Warning}},
		Tabs:   []frame.Tab{{Label: "Repos", Active: true}, {Label: "PRs"}},
		Body:   []string{"alpha", strings.Repeat("x", 100)},
		Status: "refreshed",
		Hints: []keyhint.Hint{
			{Key: "enter", What: "open", Priority: 8},
			{Key: "/", What: "search", Priority: 5},
			{Key: "s", What: "sort", Priority: 6},
			{Key: "r", What: "refresh", Priority: 1},
			{Key: "q", What: "quit", Priority: 10},
		},
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		keys     []string
		goneKeys []string
		width    int
		height   int
	}{
		{"80x24", []string{"[enter] open", "[/] search", "[s]ort", "[r]efresh", "[q]uit"}, nil, 80, 24},
		{"40x10", []string{"[q]uit"}, []string{"[r]efresh"}, 40, 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lines := render(tc.width, tc.height)
			checkShape(t, lines, tc.width, tc.height)
			checkFooter(t, lines[len(lines)-1], tc.keys, tc.goneKeys)
		})
	}
}

func checkShape(t *testing.T, lines []string, width, height int) {
	t.Helper()

	if len(lines) != height {
		t.Fatalf("drew %d lines, want %d", len(lines), height)
	}

	for i, line := range lines {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line %d is %d cells wide: %q", i, w, line)
		}
	}

	if head := lines[0]; !strings.HasPrefix(head, "gh-repo-dashboard") || !strings.HasSuffix(head, " PRs ") {
		t.Errorf("header should open on the title and end on the tabs: %q", head)
	}
}

func checkFooter(t *testing.T, foot string, keys, gone []string) {
	t.Helper()

	if !strings.HasSuffix(foot, "refreshed") {
		t.Errorf("footer should end on the status: %q", foot)
	}

	for _, k := range keys {
		if !strings.Contains(foot, k) {
			t.Errorf("footer lost %q: %q", k, foot)
		}
	}

	for _, k := range gone {
		if strings.Contains(foot, k) {
			t.Errorf("footer should have shed %q: %q", k, foot)
		}
	}
}

func render(width, height int) []string {
	s := skin.New(theme.Macchiato(), theme.Macchiato().Blue)

	return strings.Split(ansi.Strip(frame.Render(s, sample(), width, height)), "\n")
}

func stripSample() frame.Frame {
	f := sample()
	f.Strip = true
	f.Note = "/fix"
	f.Tabs = []frame.Tab{{Label: "inbox", Count: "12", Active: true}, {Label: "staged", Count: "1", Alert: true}}
	f.Panel = []string{"subject line", "facts line"}

	return f
}

func TestStripDrawsTabsLeftRuleAndPanel(t *testing.T) {
	t.Parallel()

	s := skin.New(theme.Macchiato(), theme.Macchiato().Blue)
	f := stripSample()

	const width, height = 80, 12

	lines := strings.Split(ansi.Strip(frame.Render(s, f, width, height)), "\n")
	if len(lines) != height {
		t.Fatalf("drew %d lines, want %d", len(lines), height)
	}

	head := lines[0]
	if !strings.HasPrefix(head, "gh-repo-dashboard   inbox  12    staged  1 ") || !strings.Contains(head, "3 behind") {
		t.Errorf("header should run title, tabs, then facts at the right: %q", head)
	}

	if !strings.HasPrefix(lines[1], strings.Repeat("─", 20)+"━") || !strings.HasSuffix(lines[1], "/fix") {
		t.Errorf("rule should underline the active tab and end on the note: %q", lines[1])
	}

	if foot := lines[height-1]; !strings.HasPrefix(foot, "enter open   / search") {
		t.Errorf("a strip's hints should be bare: %q", foot)
	}

	if lines[height-3] != "subject line"+strings.Repeat(" ", width-12) || lines[height-2][:10] != "facts line" {
		t.Errorf("panel should sit above the footer: %q", lines[height-3:])
	}
}

func TestClicksLandOnWhatRenderDrew(t *testing.T) {
	t.Parallel()

	s := skin.New(theme.Macchiato(), theme.Macchiato().Blue)
	f := stripSample()

	const width, height = 80, 12

	for _, c := range []struct {
		y, row int
		ok     bool
	}{{1, 0, false}, {2, 0, true}, {8, 6, true}, {9, 0, false}} {
		if row, ok := frame.BodyRow(f, height, c.y); ok != c.ok || (ok && row != c.row) {
			t.Errorf("BodyRow(y=%d) = %d, %v, want %d, %v", c.y, row, ok, c.row, c.ok)
		}
	}

	for _, c := range []struct {
		x, tab int
		ok     bool
	}{{20, 0, true}, {29, 0, true}, {34, 1, true}, {32, 0, false}, {5, 0, false}} {
		if tab, ok := frame.TabAt(s, f, width, c.x, 0); ok != c.ok || (ok && tab != c.tab) {
			t.Errorf("TabAt(x=%d) = %d, %v, want %d, %v", c.x, tab, ok, c.tab, c.ok)
		}
	}
}

func TestFooterKeepsPinnedHintsAndCutsTheStatusAtAWord(t *testing.T) {
	t.Parallel()

	s := skin.New(theme.Macchiato(), theme.Macchiato().Blue)
	f := frame.Frame{
		Status: "refreshing 14 repositories from github",
		Hints: []keyhint.Hint{
			{Key: "enter", What: "open", Priority: 9},
			{Key: "r", What: "refresh", Priority: 8},
			{Key: "?", What: "help", Pin: true},
			{Key: "q", What: "quit", Pin: true},
		},
	}

	tests := []struct {
		status string
		width  int
	}{
		{"refreshing 14 repositories from github", 80},
		{"refreshing 14…", 36},
		{"ref…", 24},
		{"", 20},
	}

	for _, tc := range tests {
		foot := ansi.Strip(frame.Render(s, f, tc.width, 3))
		foot = foot[strings.LastIndex(foot, "\n")+1:]

		for _, k := range []string{"[?] help", "[q]uit"} {
			if !strings.Contains(foot, k) {
				t.Errorf("width %d lost the pinned %q: %q", tc.width, k, foot)
			}
		}

		if tc.status != "" && !strings.HasSuffix(foot, tc.status) {
			t.Errorf("width %d footer should end on %q: %q", tc.width, tc.status, foot)
		}

		if w := lipgloss.Width(foot); w > tc.width {
			t.Errorf("width %d footer is %d cells: %q", tc.width, w, foot)
		}
	}
}

// tagged is a face that names itself around the text it draws, so a test can
// read which face drew what without decoding escapes.
func tagged(tag string) lipgloss.Style {
	return lipgloss.NewStyle().Transform(func(s string) string { return tag + "<" + s + ">" })
}

func taggedSkin() skin.Skin {
	return skin.Skin{
		Title: tagged("T"), Subtitle: tagged("S"), Badge: tagged("B"), BadgeActive: tagged("A"),
		Tab: tagged("t"), TabActive: tagged("a"), Muted: tagged("M"), Accent: tagged("C"),
		Hairline: tagged("H"), Key: tagged("K"), Warning: tagged("W"), Error: tagged("E"),
	}
}

func tabbed(strip bool) frame.Frame {
	return frame.Frame{
		Title: "gh", Strip: strip, Note: "n",
		Facts: []frame.Fact{{Text: "3", Tone: frame.Warning}},
		Tabs:  []frame.Tab{{Label: "in", Count: "12", Active: true}, {Label: "st", Count: "1", Alert: true}},
		Body:  []string{"b"},
		Hints: []keyhint.Hint{{Key: "q", What: "quit"}},
	}
}

// The active tab and its count are drawn in the active faces and every other
// tab in the idle ones, in either header. A header wider than the screen
// keeps its tabs flush left rather than shifted by a gap.
func TestRender_DrawsEachFaceWhereItBelongs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		want  []string
		f     frame.Frame
		width int
	}{
		{
			name:  "strip",
			f:     tabbed(true),
			width: 60,
			want: []string{
				"T<gh>   T<in> A<12>   S<st> B<1>                        W<3>",
				"H<────────>C<━━━━━━━━━━━>H<────────────────────────────────────> M<n>",
				"b",
				"C<q> M<quit>                                             C<>",
			},
		},
		{
			name:  "tabs at the right",
			f:     tabbed(false),
			width: 60,
			want: []string{
				"T<gh>  W<3>                           a<in> A<12> t<st> B<1>",
				"b",
				"",
				"S<>K<[q]>S<uit>                                          C<>",
			},
		},
		{
			name:  "tabs wider than the screen",
			f:     tabbed(false),
			width: 10,
			want:  []string{"a<in> A<12", "b", "", "S<>K<  C<>"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := frame.Render(taggedSkin(), tc.f, tc.width, len(tc.want))
			if want := strings.Join(tc.want, "\n"); got != want {
				t.Errorf("Render =\n%s\nwant\n%s", got, want)
			}
		})
	}

	if got := frame.Render(taggedSkin(), tabbed(true), 0, 4); got != "" {
		t.Errorf("a zero-width frame drew %q", got)
	}

	const footer = "C<q> M<quit>                                             C<>"
	if got := frame.Render(taggedSkin(), tabbed(true), 60, 1); got != footer {
		t.Errorf("a one-row frame drew %q, want only the footer %q", got, footer)
	}
}

// A tab pushed partly off the left edge is clicked from the first cell.
func TestTabAt_ClipsTabsPastTheLeftEdge(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		x, tab int
		ok     bool
	}{{0, 1, true}, {9, 1, true}, {10, 0, false}} {
		if tab, ok := frame.TabAt(taggedSkin(), tabbed(false), 10, c.x, 0); ok != c.ok || (ok && tab != c.tab) {
			t.Errorf("TabAt(x=%d) = %d, %v, want %d, %v", c.x, tab, ok, c.tab, c.ok)
		}
	}
}

func TestLit_PadsAndKeepsTheBackgroundThroughResets(t *testing.T) {
	t.Parallel()

	red := lipgloss.NewStyle().Background(lipgloss.Color("1"))
	tests := []struct {
		face             lipgloss.Style
		name, text, want string
		width            int
	}{
		{name: "an unstyled face draws text as is", text: "ab", width: 5, face: lipgloss.NewStyle(), want: "ab"},
		{name: "one cell short", text: "ab", width: 3, face: red, want: "\x1b[41mab \x1b[m"},
		{name: "exact", text: "ab", width: 2, face: red, want: "\x1b[41mab\x1b[m"},
		{name: "too wide is not cut", text: "abc", width: 2, face: red, want: "\x1b[41mabc\x1b[m"},
		{
			name: "both reset spellings reopen", text: "a\x1b[mb\x1b[0mc", width: 3, face: red,
			want: "\x1b[41ma\x1b[m\x1b[41mb\x1b[0m\x1b[41mc\x1b[m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := frame.Lit(tc.text, tc.width, tc.face); got != tc.want {
				t.Errorf("Lit(%q, %d) = %q, want %q", tc.text, tc.width, got, tc.want)
			}
		})
	}
}
