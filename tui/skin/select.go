package skin

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// The selected row's policy, which every tool inherits through these two calls
// rather than composing a face with Selection itself.
//
// Where Selection is a tint (truecolor, 256 colors) a cell keeps its own colors
// on the tinted background. Where it is reverse video (16 colors, NO_COLOR) a
// cell's colors are dropped inside the row: reversing a colored cell swaps its
// foreground into the background, so a status letter or a +3 inside the row
// would draw as a small colored block in a white bar. The row reads as one bar
// and the weights (bold, underline) still mark what they marked.

// Selected is the face a cell inside the selected row is drawn in: st on the
// selection's tint, or st without its colors in reverse video where the
// selection is reverse video.
func (s Skin) Selected(st lipgloss.Style) lipgloss.Style {
	if s.Selection.GetReverse() {
		return st.UnsetForeground().UnsetBackground().Reverse(true)
	}

	return st.Inherit(s.Selection)
}

// Select draws text, a row already carrying its own faces, as the selected row
// width cells wide: Lit with the Selection face.
func (s Skin) Select(text string, width int) string { return Lit(text, width, s.Selection) }

// Lit pads text to width and draws it on face's background, restarting the
// background after every reset inside text so styled spans do not punch holes
// in it. A reverse-video face drops the colors text carries first, for the
// reason Selected does, so the row is one even bar.
func Lit(text string, width int, face lipgloss.Style) string {
	on, _, _ := strings.Cut(face.Render("x"), "x")
	if on == "" {
		return text
	}

	if face.GetReverse() {
		text = ansi.Strip(text)
	}

	if over := width - ansi.StringWidth(text); over > 0 {
		text += strings.Repeat(" ", over)
	}

	for _, reset := range []string{"\x1b[m", "\x1b[0m"} {
		text = strings.ReplaceAll(text, reset, reset+on)
	}

	return on + text + "\x1b[m"
}
