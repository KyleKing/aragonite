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
	if !strings.HasPrefix(head, "gh-repo-dashboard   inbox 12    staged 1 ") || !strings.Contains(head, "3 behind") {
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
	}{{20, 0, true}, {29, 0, false}, {32, 1, true}, {5, 0, false}} {
		if tab, ok := frame.TabAt(s, f, width, c.x, 0); ok != c.ok || (ok && tab != c.tab) {
			t.Errorf("TabAt(x=%d) = %d, %v, want %d, %v", c.x, tab, ok, c.tab, c.ok)
		}
	}
}
