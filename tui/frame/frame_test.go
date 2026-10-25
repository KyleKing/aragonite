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
