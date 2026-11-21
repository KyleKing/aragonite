package overlay_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/overlay"
)

func testStyles() overlay.Styles {
	return overlay.Styles{
		Frame:   lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 3),
		Elision: lipgloss.NewStyle(),
	}
}

// lipgloss.Place centers each line against its own width, so a block of
// differing line lengths staggers: every line is padded to the widest first.
// Content larger than the room inside the frame is clipped, and an elision
// says lines were dropped. A frame with no room at all draws content as is.
func TestCenter_FramesClipsAndCentersTheBlockAsOne(t *testing.T) {
	t.Parallel()

	styles := overlay.Styles{Frame: lipgloss.NewStyle().Border(lipgloss.NormalBorder())}
	tests := []struct {
		name          string
		content       string
		want          string
		width, height int
	}{
		{
			name: "fits", content: "ab\nlonger\nc", width: 10, height: 5,
			want: " ┌──────┐ \n │ab    │ \n │longer│ \n │c     │ \n └──────┘ ",
		},
		{
			name: "too tall", content: "1\n2\n3\n4", width: 7, height: 5,
			want: "  ┌─┐  \n  │1│  \n  │2│  \n  │…│  \n  └─┘  ",
		},
		{name: "too wide", content: "abcdefghij", width: 6, height: 3, want: "┌────┐\n│abc…│\n└────┘"},
		{name: "one cell of room", content: "abc", width: 3, height: 3, want: "┌─┐\n│a│\n└─┘"},
		{name: "one row of room", content: "abc\ndef", width: 4, height: 3, want: "┌─┐ \n│…│ \n└─┘ "},
		{
			name: "no columns of room", content: "abc\nd", width: 2, height: 6,
			want: "     \n┌───┐\n│abc│\n│d  │\n└───┘\n     ",
		},
		{name: "no rows of room", content: "abc\nd", width: 6, height: 2, want: "┌───┐ \n│abc│ \n│d  │ \n└───┘ "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ansi.Strip(overlay.Center(tc.content, tc.width, tc.height, styles)); got != tc.want {
				t.Errorf("Center(%q, %d, %d) =\n%s\nwant\n%s", tc.content, tc.width, tc.height, got, tc.want)
			}
		})
	}

	padded := testStyles()
	if got := overlay.ContentWidth(80, padded); got != 72 {
		t.Errorf("content has %d cells inside a bordered, padded 80-cell frame, want 72", got)
	}

	if got := overlay.ContentHeight(24, padded); got != 20 {
		t.Errorf("content has %d rows inside the frame, want 20", got)
	}
}

func TestBottomRight_LandsFlushInTheCornerAndLeavesTheRestAlone(t *testing.T) {
	t.Parallel()

	panel := keyhint.PanelStyles{Frame: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)}
	hints := []keyhint.Hint{
		{Key: "a", What: "all repos", Count: "12", On: true},
		{Key: "d", What: "dirty only", Count: "3"},
		{Key: "x", What: "clear filters"},
	}

	tests := []struct {
		name          string
		width, height int
	}{
		{name: "wide", width: 80, height: 24},
		{name: "small", width: 40, height: 10},
		{name: "narrower than the panel", width: 14, height: 6},
		{name: "shorter than the panel", width: 30, height: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base := make([]string, tc.height)
			for i := range base {
				base[i] = strings.Repeat(string(rune('a'+i%26)), tc.width)
			}

			box := keyhint.Panel(panel, hints, tc.width)
			got := overlay.BottomRight(base, box, tc.width)

			if len(got) != tc.height {
				t.Fatalf("drew %d rows over a %d-row base", len(got), tc.height)
			}

			assertOutsideUntouched(t, base, got, box, tc.width)

			if last := ansi.Strip(got[tc.height-1]); !strings.HasSuffix(last, "╯") {
				t.Errorf("bottom row %q does not end in the panel's corner", last)
			}
		})
	}
}

func assertOutsideUntouched(t *testing.T, base, got, box []string, width int) {
	t.Helper()

	boxWidth := 0
	for _, l := range box {
		boxWidth = max(boxWidth, ansi.StringWidth(l))
	}

	boxWidth = min(boxWidth, width)
	top := len(base) - min(len(box), len(base))

	for i, line := range got {
		if w := ansi.StringWidth(line); w != width {
			t.Fatalf("row %d is %d cells wide, want %d", i, w, width)
		}

		kept := ansi.Strip(ansi.Cut(line, 0, width-boxWidth))
		if i < top {
			kept = ansi.Strip(line)
		}

		if want := base[i][:len(kept)]; kept != want {
			t.Errorf("row %d outside the panel is %q, want %q", i, kept, want)
		}
	}
}

func TestAtCoversWhatBlitDraws(t *testing.T) {
	t.Parallel()

	r := overlay.At([]string{"abc", "abcde"}, 4, 2)

	for _, c := range []struct {
		x, y int
		in   bool
	}{{4, 2, true}, {8, 3, true}, {9, 3, false}, {3, 2, false}, {4, 4, false}} {
		if got := r.Contains(c.x, c.y); got != c.in {
			t.Errorf("Contains(%d, %d) = %v, want %v", c.x, c.y, got, c.in)
		}
	}
}

func TestOver_DimsTheBackgroundAroundTheContent(t *testing.T) {
	t.Parallel()

	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	styles := testStyles()
	styles.Dim = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	tests := []struct {
		name string
		row  string
	}{
		{name: "ascii", row: strings.Repeat("abcdefghij", 6)},
		{name: "wide runes straddling the edges", row: strings.Repeat("\u4e16", 30)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			const width, height = 60, 12

			base := strings.TrimSuffix(strings.Repeat(red.Render(tc.row)+"\n", height), "\n")
			got := overlay.Over(base, "hello", width, height, styles)

			if strings.Contains(got, "\x1b[1m") || strings.Contains(got, "31m") {
				t.Errorf("the background kept its own styling: %q", got)
			}

			lines := strings.Split(got, "\n")
			if len(lines) != height {
				t.Fatalf("drew %d rows, want %d", len(lines), height)
			}

			assertWidths(t, lines, width)

			if !strings.Contains(lines[0], "\x1b[2") {
				t.Errorf("an uncovered row is not faint: %q", lines[0])
			}

			if !strings.Contains(ansi.Strip(got), "hello") {
				t.Error("the content is missing")
			}
		})
	}
}

func assertWidths(t *testing.T, lines []string, width int) {
	t.Helper()

	for i, line := range lines {
		if w := ansi.StringWidth(line); w != width {
			t.Errorf("row %d is %d cells wide, want %d", i, w, width)
		}
	}
}
