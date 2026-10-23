package card_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/card"
)

func TestRenderRowsAreExactWidthAndOneRowPerLinePlusEdges(t *testing.T) {
	t.Parallel()

	styles := card.Styles{
		Edge: lipgloss.NewStyle().Foreground(lipgloss.Color("#8087a2")),
		Fill: lipgloss.NewStyle().Background(lipgloss.Color("#101216")),
	}
	long := strings.Repeat("x", 80)

	tests := []struct {
		name  string
		c     card.Card
		width int
	}{
		{name: "title only", c: card.Card{Title: "tests", Body: []string{"ok"}}, width: 24},
		{name: "with note", c: card.Card{Title: "tests", Note: "3 of 4", Body: []string{"ok"}}, width: 24},
		{name: "with foot", c: card.Card{Title: "tests", Body: []string{"ok"}, Foot: "main"}, width: 24},
		{
			name:  "with note and foot",
			c:     card.Card{Title: "tests", Note: "3 of 4", Body: []string{"ok"}, Foot: "main"},
			width: 24,
		},
		{name: "no body", c: card.Card{Title: "tests"}, width: 24},
		{name: "over-long title and note", c: card.Card{Title: long, Note: long, Body: []string{"ok"}}, width: 30},
		{name: "over-long foot", c: card.Card{Title: "tests", Body: []string{"ok"}, Foot: long}, width: 30},
		{name: "over-long body line", c: card.Card{Body: []string{long}}, width: 20},
		{name: "styled body line", c: card.Card{Body: []string{"\x1b[31mred\x1b[0m text"}}, width: 20},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rows := tc.c.Render(styles, tc.width)

			if len(rows) != len(tc.c.Body)+2 {
				t.Fatalf("got %d rows for %d body lines, want %d", len(rows), len(tc.c.Body), len(tc.c.Body)+2)
			}

			for i, row := range rows {
				if got := lipgloss.Width(row); got != tc.width {
					t.Errorf("row %d is %d cells wide, want %d: %q", i, got, tc.width, row)
				}
			}
		})
	}
}

func TestRenderDrawsTheEdgesAndCaptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		c     card.Card
		want  []string
		width int
	}{
		{
			name:  "title alone",
			c:     card.Card{Title: "tests", Body: []string{"ok"}},
			width: 16,
			want: []string{
				"╭─ tests " + strings.Repeat("─", 6) + "╮",
				"│ ok" + strings.Repeat(" ", 10) + " │",
				"╰" + strings.Repeat("─", 14) + "╯",
			},
		},
		{
			name:  "note and foot",
			c:     card.Card{Title: "tests", Note: "3 of 4", Body: []string{"ok"}, Foot: "main"},
			width: 24,
			want: []string{
				"╭─ tests " + strings.Repeat("─", 5) + " 3 of 4 ─╮",
				"│ ok" + strings.Repeat(" ", 18) + " │",
				"╰─ main " + strings.Repeat("─", 14) + "─╯",
			},
		},
		{
			name:  "over-long title leaves no room for the note",
			c:     card.Card{Title: "a very long title", Note: "3 of 4", Body: []string{"ok"}},
			width: 24,
			want: []string{
				"╭─ a very long ti… ────╮",
				"│ ok" + strings.Repeat(" ", 18) + " │",
				"╰" + strings.Repeat("─", 22) + "╯",
			},
		},
		{
			name:  "over-long note is cut",
			c:     card.Card{Title: "tests", Note: "a long note too", Body: []string{"ok"}},
			width: 24,
			want: []string{
				"╭─ tests ─ a long no… ─╮",
				"│ ok" + strings.Repeat(" ", 18) + " │",
				"╰" + strings.Repeat("─", 22) + "╯",
			},
		},
		{
			name:  "over-long foot is cut",
			c:     card.Card{Title: "tests", Body: []string{"ok"}, Foot: "build and test"},
			width: 16,
			want: []string{
				"╭─ tests ──────╮",
				"│ ok" + strings.Repeat(" ", 10) + " │",
				"╰─ build and… ─╯",
			},
		},
		{
			name:  "over-long body line is cut to the inner width",
			c:     card.Card{Body: []string{"this line is much too long"}},
			width: 16,
			want: []string{
				"╭" + strings.Repeat("─", 14) + "╮",
				"│ this line i… │",
				"╰" + strings.Repeat("─", 14) + "╯",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.c.Render(card.Styles{}, tc.width)

			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestRenderReopensTheFillAfterAnInnerReset(t *testing.T) {
	t.Parallel()

	fill := lipgloss.NewStyle().Background(lipgloss.Color("#101216"))
	styles := card.Styles{Edge: lipgloss.NewStyle(), Fill: fill}

	rows := card.Card{Body: []string{"a\x1b[31mred\x1b[0m b"}}.Render(styles, 16)

	const marker = "x"

	bg, _, _ := strings.Cut(fill.Render(marker), marker)
	if !strings.Contains(rows[1], "\x1b[0m"+bg+" b") {
		t.Errorf("the background is not reopened after the body's own reset: %q", rows[1])
	}
}
