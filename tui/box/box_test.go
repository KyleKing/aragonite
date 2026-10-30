package box_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/box"
)

func styles() box.Styles {
	return box.Styles{
		Idle:    lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#8087a2")),
		Focused: lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("#8aadf4")),
	}
}

func TestRenderIsExactlyItsRectangleForAnyContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "empty", content: ""},
		{name: "one short line", content: "hi"},
		{name: "a line wider than the box", content: strings.Repeat("wide ", 40)},
		{name: "more lines than rows", content: strings.Repeat("row\n", 40)},
		{name: "styled content", content: "\x1b[1mTitle\x1b[0m\n\x1b[2m[?] help\x1b[0m"},
		{name: "wide runes", content: strings.Repeat("\u65e5\u672c\u8a9e", 20)},
		{name: "both over at once", content: strings.Repeat(strings.Repeat("x", 90)+"\n", 30)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, size := range [][2]int{{20, 5}, {30, 3}, {80, 24}} {
				for _, focused := range []bool{false, true} {
					assertRect(t, box.Render(styles(), size[0], size[1], focused, tt.content), size[0], size[1])
				}
			}
		})
	}
}

func assertRect(t *testing.T, rendered string, width, height int) {
	t.Helper()

	lines := strings.Split(ansi.Strip(rendered), "\n")
	if len(lines) != height {
		t.Fatalf("a %dx%d box drew %d rows", width, height, len(lines))
	}

	for i, line := range lines {
		if got := ansi.StringWidth(line); got != width {
			t.Errorf("row %d is %d cells wide, want %d: %q", i, got, width, line)
		}
	}

	for _, edge := range []string{lines[0], lines[height-1]} {
		runes := []rune(edge)
		if runes[0] == ' ' || runes[len(runes)-1] == ' ' {
			t.Errorf("a corner is blank, so the box lost a border:\n%s", ansi.Strip(rendered))

			break
		}
	}
}

func TestRenderDrawsTheFocusedBorderAndCutsContent(t *testing.T) {
	t.Parallel()

	content := "one\ntwo is far too long\nthree"

	tests := []struct {
		name    string
		want    string
		focused bool
	}{
		{
			name: "idle",
			want: "╭────────╮\n│one     │\n│two is …│\n╰────────╯",
		},
		{
			name:    "focused",
			focused: true,
			want:    "┏━━━━━━━━┓\n┃one     ┃\n┃two is …┃\n┗━━━━━━━━┛",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ansi.Strip(box.Render(styles(), 10, 4, tt.focused, content))
			if got != tt.want {
				t.Errorf("Render =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}
