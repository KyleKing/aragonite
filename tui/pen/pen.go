// Package pen draws text in a lipgloss style without running lipgloss's renderer on every call.
//
// Style.Render re-reads every property of the style, builds the escape sequence, splits the
// text into lines, and measures it, which is most of what a full-screen frame costs when a
// row is a dozen styled runs. A Pen does that once, when it is made, and afterwards draws a
// run as the style's opening sequence, the text, and a reset: byte for byte what Render
// answers for a single line, at the cost of a string concatenation.
//
// Only a style whose Render is exactly that shape gets the fast path. One with a width,
// padding, a border, a transform, a link, or styled spaces keeps calling Render, as does any
// text holding a line break, so a Pen is never wrong, only sometimes no faster.
package pen

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// reset is the sequence lipgloss closes a styled run with.
const reset = "\x1b[m"

// tab is how lipgloss draws a tab when the style does not set its own tab width.
const tab = "    "

// Pen draws text in one style. The zero Pen draws text unstyled, with tabs expanded the way
// an unstyled lipgloss.Style would.
type Pen struct {
	style   lipgloss.Style
	opening string
	closing string
	direct  bool
	set     bool
}

// Of returns a Pen drawing in s. It renders a few probes through s to learn its escape
// sequence, so make Pens once per style, not once per run.
func Of(s lipgloss.Style) Pen {
	p := Pen{style: s, set: true}

	if s.GetWidth() > 0 || s.GetHeight() > 0 || s.GetMaxWidth() > 0 || s.GetMaxHeight() > 0 ||
		s.GetTransform() != nil {
		return p
	}

	probe := s.Render("x")

	switch {
	case probe == "x":
	case strings.HasSuffix(probe, "x"+reset):
		p.opening, p.closing = strings.TrimSuffix(probe, "x"+reset), reset
	default:
		return p
	}

	// A second probe catches what one character cannot: padding, styled spaces, and a tab
	// width of the style's own.
	for _, text := range []string{"a b\tc", ""} {
		if s.Render(text) != p.opening+expand(text)+p.closing {
			return p
		}
	}

	p.direct = true

	return p
}

// Style is the style the Pen draws in.
func (p Pen) Style() lipgloss.Style {
	return p.style
}

// Render answers what p's style renders text as.
func (p Pen) Render(text string) string {
	if !p.fast(text) {
		return p.slow(text)
	}

	if p.opening == "" {
		return expand(text)
	}

	return p.opening + expand(text) + p.closing
}

// Write appends what Render answers for text to b, without building the run as its own string.
func (p Pen) Write(b *strings.Builder, text string) {
	if !p.fast(text) {
		b.WriteString(p.slow(text))

		return
	}

	b.WriteString(p.opening)

	if strings.IndexByte(text, '\t') < 0 {
		b.WriteString(text)
	} else {
		b.WriteString(expand(text))
	}

	b.WriteString(p.closing)
}

// Spaces appends n spaces in p's style, which is how a row's wash is carried to the pane's edge.
func (p Pen) Spaces(b *strings.Builder, n int) {
	if n <= 0 {
		return
	}

	if !p.direct {
		b.WriteString(p.slow(strings.Repeat(" ", n)))

		return
	}

	b.WriteString(p.opening)

	for n > len(blanks) {
		b.WriteString(blanks)
		n -= len(blanks)
	}

	b.WriteString(blanks[:n])
	b.WriteString(p.closing)
}

const blanks = "                                                                " +
	"                                                                "

func (p Pen) fast(text string) bool {
	if !p.set {
		return !strings.ContainsAny(text, "\r\n")
	}

	return p.direct && !strings.ContainsAny(text, "\r\n")
}

func (p Pen) slow(text string) string {
	if !p.set {
		return lipgloss.NewStyle().Render(text)
	}

	return p.style.Render(text)
}

func expand(text string) string {
	if strings.IndexByte(text, '\t') < 0 {
		return text
	}

	return strings.ReplaceAll(text, "\t", tab)
}
