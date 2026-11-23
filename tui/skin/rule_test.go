package skin_test

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

func TestRule(t *testing.T) {
	t.Parallel()

	p := theme.Macchiato()
	s := skin.New(p, p.Mauve)

	tests := []struct {
		name  string
		title string
		count string
		want  string
		width int
	}{
		{name: "fills the width", title: "Files", count: "10", width: 20, want: "── Files 10 ────────"},
		{name: "no count", title: "Stops", count: "", width: 14, want: "── Stops ─────"},
		{name: "too narrow cuts the name", title: "Chapters", count: "3", width: 8, want: "── Chapt"},
		{name: "exactly the name leaves no tail", title: "Files", count: "", width: 9, want: "── Files"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := s.Rule(tt.title, tt.count, tt.width)
			if plain := ansi.Strip(got); plain != tt.want {
				t.Errorf("got %q, want %q", plain, tt.want)
			}

			if w := lipgloss.Width(got); w > tt.width {
				t.Errorf("width %d overruns %d", w, tt.width)
			}
		})
	}
}

func TestDiffFaces(t *testing.T) {
	p := theme.Macchiato()

	t.Run("color draws a bar on both sides", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")

		d := skin.Diff(p, true)
		if d.Sign(true) != skin.SignBar || d.Sign(false) != skin.SignBar {
			t.Error("want the sign bar")
		}
	})

	t.Run("NO_COLOR draws plus and minus", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")

		d := skin.Diff(p, true)
		if d.Sign(true) != "+" || d.Sign(false) != "−" {
			t.Errorf("got %q %q", d.Sign(true), d.Sign(false))
		}
	})

	//nolint:paralleltest // siblings set NO_COLOR
	t.Run("the mark is deeper than the wash on either profile", func(t *testing.T) {
		for _, millions := range []bool{true, false} {
			d := skin.Diff(p, millions)
			if theme.Contrast(d.AddMark, p.Base) <= theme.Contrast(d.AddWash, p.Base) ||
				theme.Contrast(d.RemoveMark, p.Base) <= theme.Contrast(d.RemoveWash, p.Base) {
				t.Errorf("millions=%v: mark should stand further off the base than the wash", millions)
			}
		}
	})
}

func TestFileBandIsReverseUnderNoColor(t *testing.T) {
	t.Parallel()

	p := theme.Macchiato()
	if !skin.Build(p, p.Mauve, true).FileBand.GetReverse() {
		t.Error("want reverse video")
	}

	if skin.Build(p, p.Mauve, false).FileBand.GetReverse() {
		t.Error("color draws a tint, not reverse")
	}
}
