package split_test

import (
	"slices"
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

func TestMainWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		spec      split.Spec
		placement split.Placement
		width     int
		want      int
	}{
		{name: "even split", spec: split.Spec{}, placement: split.Side, width: 101, want: 50},
		{
			name:      "even split around a padded rule",
			spec:      split.Spec{Pad: 2},
			placement: split.Side,
			width:     101,
			want:      48,
		},
		{name: "capped", spec: split.Spec{MainMax: 30}, placement: split.Side, width: 101, want: 30},
		{name: "a cap of one", spec: split.Spec{MainMax: 1}, placement: split.Side, width: 101, want: 1},
		{
			name:      "the cap yields to the rule",
			spec:      split.Spec{MainMax: 100, Pad: 1},
			placement: split.Side,
			width:     50,
			want:      47,
		},
		{name: "below is full width", spec: split.Spec{MainMax: 30}, placement: split.Below, width: 101, want: 101},
		{name: "none is full width", spec: split.Spec{MainMax: 30}, placement: split.None, width: 101, want: 101},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.spec.MainWidth(tc.placement, tc.width); got != tc.want {
				t.Errorf("MainWidth = %d, want %d", got, tc.want)
			}
		})
	}
}

// Each block is cut and padded to its own share, and a padded rule sits
// between side-by-side blocks.
func TestRender_JoinsTheBlocks(t *testing.T) {
	t.Parallel()

	small := split.Spec{SideMin: 10, MainMax: 4, BelowPercent: 50, Pad: 1}
	main := func(int, int) []string { return []string{"main block", "m"} }
	detail := func(int, int) []string { return []string{"detail block", "d", "x"} }
	rule := split.Styles{Rule: lipgloss.NewStyle().Transform(func(s string) string { return "[" + s + "]" })}

	tests := []struct {
		name          string
		want          []string
		placement     split.Placement
		width, height int
	}{
		{
			name:      "side",
			placement: split.Side,
			width:     13,
			height:    3,
			want:      []string{"main [│] detail", "m    [│] d     ", "     [│] x     "},
		},
		{
			name:      "below",
			placement: split.Below,
			width:     6,
			height:    4,
			want:      []string{"main b", "m     ", "detail", "d     "},
		},
		{name: "none", placement: split.None, width: 6, height: 3, want: []string{"main b", "m     ", "      "}},
		{name: "no width", placement: split.None, width: 0, height: 3, want: nil},
		{name: "no height", placement: split.Side, width: 13, height: 0, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := small.Render(tc.placement, rule, tc.width, tc.height, main, detail)
			if !slices.Equal(got, tc.want) {
				t.Errorf("Render =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}
