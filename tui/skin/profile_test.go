package skin_test

import (
	"fmt"
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

// grounds is every background a skin and its diff faces paint behind text that
// is meant to be read through it. The accent chips (TabActive, BadgeActive) are
// left out: they are meant to be a solid block of the accent.
func grounds(s skin.Skin, d skin.DiffFaces) map[string]color.Color {
	return map[string]color.Color{
		"selection":   s.Selection.GetBackground(),
		"band":        s.Band.GetBackground(),
		"card":        s.Card.GetBackground(),
		"panel":       s.Panel.GetBackground(),
		"file band":   s.FileBand.GetBackground(),
		"tab":         s.Tab.GetBackground(),
		"badge":       s.Badge.GetBackground(),
		"add wash":    d.AddWash,
		"remove wash": d.RemoveWash,
		"add mark":    d.AddMark,
		"remove mark": d.RemoveMark,
	}
}

type flavor struct {
	palette theme.Palette
	name    string
}

func flavors() []flavor {
	return []flavor{
		{name: "ink", palette: theme.Ink()},
		{name: "macchiato", palette: theme.Macchiato()},
		{name: "latte", palette: theme.Latte()},
	}
}

// TestGroundsAreGreyIn256Colors pins the reason the 256-color path exists: the
// cube's dark entries are saturated blocks (22 #005f00, 52 #5f0000, 17 #00005f),
// and a background the renderer quantizes on its own lands on them. Every
// background a skin paints must be a grey there, and each role keeps its place
// on the ramp relative to the others.
func TestGroundsAreGreyIn256Colors(t *testing.T) {
	t.Parallel()

	for _, f := range flavors() {
		for _, accent := range []color.Color{f.palette.Lavender, f.palette.Green, f.palette.Peach} {
			t.Run(fmt.Sprintf("%s/%v", f.name, accent), func(t *testing.T) {
				t.Parallel()

				s := skin.For(f.palette, accent, colorprofile.ANSI256)
				d := skin.DiffFor(f.palette, colorprofile.ANSI256)

				for role, bg := range grounds(s, d) {
					got := colorprofile.ANSI256.Convert(bg)
					if !grey(got) {
						t.Errorf("%s is %v (%s) in 256 colors, want a grey", role, got, hex(got))
					}
				}

				if d.AddWash == d.AddMark {
					t.Errorf("a changed line's wash and mark are both %v", d.AddWash)
				}

				if band := s.FileBand.GetBackground(); band == d.AddWash {
					t.Errorf("a file's heading and a changed line's wash are both %v", band)
				}
			})
		}
	}
}

// TestGroundsAreDroppedIn16Colors checks that 16 colors paints nothing behind
// text, where the nearest match is black or a mid grey that every theme draws
// differently, and that the cursor row falls back to reverse video.
func TestGroundsAreDroppedIn16Colors(t *testing.T) {
	t.Parallel()

	for _, f := range flavors() {
		s := skin.For(f.palette, f.palette.Lavender, colorprofile.ANSI)
		d := skin.DiffFor(f.palette, colorprofile.ANSI)

		for role, bg := range grounds(s, d) {
			if _, none := bg.(lipgloss.NoColor); !none && bg != nil {
				t.Errorf("%s: %s is %v in 16 colors, want none", f.name, role, bg)
			}
		}

		if !s.Selection.GetReverse() || !s.Band.GetReverse() {
			t.Errorf("%s: the cursor row is not reverse video in 16 colors", f.name)
		}
	}
}

// TestGroundsAreQuietTintsInTruecolor checks the blended tints: each lifts off
// the frame enough to see and stays quiet enough that body text keeps 4.5:1 on
// it, and the mark is deeper than the wash it sits in.
func TestGroundsAreQuietTintsInTruecolor(t *testing.T) {
	t.Parallel()

	const (
		floor = 4.5
		// quietest and loudest are CIEDE2000 distances from the frame: 1 is about
		// where a difference can be seen side by side, and past 30 a background
		// reads as a block of color rather than a tint.
		quietest = 1.0
		loudest  = 30.0
	)

	for _, f := range flavors() {
		s := skin.For(f.palette, f.palette.Lavender, colorprofile.TrueColor)
		d := skin.DiffFor(f.palette, colorprofile.TrueColor)

		for role, bg := range grounds(s, d) {
			if got := theme.Contrast(f.palette.Text, bg); got < floor {
				t.Errorf("%s: text on %s is %.2f:1, want at least %.1f:1", f.name, role, got, floor)
			}

			if got := distance(f.palette.Base, bg); got < quietest || got > loudest {
				t.Errorf("%s: %s sits %.1f off the frame, want %.0f to %.0f", f.name, role, got, quietest, loudest)
			}
		}

		if distance(f.palette.Base, d.AddMark) <= distance(f.palette.Base, d.AddWash) {
			t.Errorf("%s: the add mark is no deeper than its wash", f.name)
		}
	}
}

// TestEdgeIsVisible keeps a raised block's border legible: a hairline at under
// 2:1 against the frame disappears on most displays.
func TestEdgeIsVisible(t *testing.T) {
	t.Parallel()

	const floor = 2.1

	for _, f := range flavors() {
		edge := skin.New(f.palette, f.palette.Lavender).Edge.GetForeground()
		if got := theme.Contrast(edge, f.palette.Base); got < floor {
			t.Errorf("%s: the edge is %.2f:1 against the frame, want at least %.1f:1", f.name, got, floor)
		}
	}
}

func grey(c color.Color) bool {
	i, ok := c.(ansi.IndexedColor)

	return ok && (i >= 232 || i == 16 || i == 231)
}

func hex(c color.Color) string {
	cf, _ := colorful.MakeColor(c)

	return cf.Hex()
}

// distance is the CIEDE2000 difference between a and b, on the scale where 1 is
// about the smallest difference visible side by side.
func distance(a, b color.Color) float64 {
	const scale = 100

	ca, _ := colorful.MakeColor(a)
	cb, _ := colorful.MakeColor(b)

	return scale * ca.DistanceCIEDE2000(cb)
}

// In 256 colors the side of a line is said by its sign alone, so both signs keep
// their hue there: Latte's red answers a grey through colorprofile's own match.
func TestDiffSignsKeepTheirHueIn256Colors(t *testing.T) {
	t.Parallel()

	for name, p := range map[string]theme.Palette{"latte": theme.Latte(), "macchiato": theme.Macchiato()} {
		d := skin.DiffFor(p, colorprofile.ANSI256)

		for side, face := range map[string]lipgloss.Style{"added": d.AddSign, "removed": d.RemoveSign} {
			got := colorprofile.ANSI256.Convert(face.GetForeground())
			if r, g, b, _ := got.RGBA(); r == g && g == b {
				t.Errorf("%s %s sign is the grey %v", name, side, got)
			}
		}
	}
}
