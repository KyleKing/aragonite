package split_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/split"
)

var spec = split.Spec{SideMin: 140, MainMax: 100, MinWidth: 100, MinHeight: 15, BelowPercent: 60}

func TestPlace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		width, height int
		open          bool
		want          split.Placement
	}{
		{"side at threshold closed", 140, 40, false, split.Side},
		{"side above threshold open", 200, 40, true, split.Side},
		{"below one under threshold open", 139, 40, true, split.Below},
		{"closed one under threshold", 139, 40, false, split.None},
		{"below at min width", 100, 40, true, split.Below},
		{"none under min width", 99, 40, true, split.None},
		{"below at min height", 120, 15, true, split.Below},
		{"none under min height", 120, 14, true, split.None},
		{"side ignores height floor", 140, 3, false, split.Side},
		{"empty area", 0, 0, true, split.None},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := spec.Place(tc.width, tc.height, tc.open); got != tc.want {
				t.Fatalf("Place(%d, %d, %v) = %d, want %d", tc.width, tc.height, tc.open, got, tc.want)
			}
		})
	}
}

func TestRenderExactSize(t *testing.T) {
	t.Parallel()

	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Bold(true)
	wide := strings.Repeat("\u754cx", 80)

	main := func(_, _ int) []string {
		return []string{styled.Render(wide), "short", styled.Render("é")}
	}
	detail := func(_, h int) []string {
		lines := make([]string, h+5)
		for i := range lines {
			lines[i] = styled.Render(wide)
		}

		return lines
	}

	tests := []struct {
		name          string
		placement     split.Placement
		width, height int
	}{
		{"side", split.Side, 160, 20},
		{"side narrow main", split.Side, 141, 8},
		{"below", split.Below, 120, 30},
		{"none", split.None, 90, 12},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			styles := split.Styles{Rule: lipgloss.NewStyle()}
			lines := spec.Render(tc.placement, styles, tc.width, tc.height, main, detail)
			if len(lines) != tc.height {
				t.Fatalf("got %d lines, want %d", len(lines), tc.height)
			}

			for i, line := range lines {
				if got := lipgloss.Width(line); got != tc.width {
					t.Fatalf("line %d is %d cells, want %d: %q", i, got, tc.width, line)
				}
			}
		})
	}
}

func TestRenderSizesRenderers(t *testing.T) {
	t.Parallel()

	type size struct{ w, h int }

	record := func(into *size) split.Renderer {
		return func(w, h int) []string {
			*into = size{w, h}

			return nil
		}
	}

	tests := []struct {
		name         string
		placement    split.Placement
		wantMain     size
		wantDetail   size
		detailCalled bool
	}{
		{"side", split.Side, size{100, 40}, size{59, 40}, true},
		{"below", split.Below, size{160, 16}, size{160, 24}, true},
		{"none", split.None, size{160, 40}, size{}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotMain, gotDetail size

			spec.Render(tc.placement, split.Styles{}, 160, 40, record(&gotMain), record(&gotDetail))

			if gotMain != tc.wantMain {
				t.Errorf("main sized %v, want %v", gotMain, tc.wantMain)
			}

			if gotDetail != tc.wantDetail {
				t.Errorf("detail sized %v, want %v", gotDetail, tc.wantDetail)
			}
		})
	}
}

func TestMainWidthEvenSplit(t *testing.T) {
	t.Parallel()

	even := split.Spec{SideMin: 100}
	if got := even.MainWidth(split.Side, 101); got != 50 {
		t.Fatalf("MainWidth = %d, want 50", got)
	}
}
