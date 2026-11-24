// Package empty draws the card a screen shows where it has nothing to list: what is empty, why,
// and two or three keys out. An empty list with no explanation reads as broken, and a reader who
// does not know the next key is stuck; the card answers both in the place they are looking.
//
// The same card is a tool's first-run welcome: the handful of keys that unlock the rest.
package empty

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/card"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/table"
)

// Empty is what a card says.
type Empty struct {
	// Title names what is empty, on the card's top edge: "No changes in wkwyyxvq".
	Title string
	// Why is a sentence or two on why, and what would fill it.
	Why string
	// Hints are the keys out, drawn bare in one line: "ctrl+l reload  q quit".
	Hints []keyhint.Hint
}

// MaxWidth is the widest a card draws, so its prose stays a readable measure on a wide screen.
const MaxWidth = 64

const (
	margin = 4
	halves = 2
)

// Block is the card alone, as wide as its content needs up to MaxWidth and the frame's width less
// a margin: the title on the top edge, the reason wrapped inside, then the keys out.
func Block(s skin.Skin, e Empty, width int) []string {
	faces := keyhint.Styles{Key: s.Key, Text: s.Muted, Off: s.Muted, Bare: true}
	hints := keyhint.Line(faces, e.Hints)

	want := max(lipgloss.Width(e.Title)+margin*halves, lipgloss.Width(hints), lipgloss.Width(e.Why)) + margin
	w := max(1, min(want, MaxWidth, width-margin))
	inner := card.InnerWidth(w)

	var body []string
	for _, l := range wrap(e.Why, inner) {
		body = append(body, s.Body.Render(l))
	}

	if hints != "" {
		if len(body) > 0 {
			body = append(body, "")
		}

		body = append(body, keyhint.Fit(faces, e.Hints, inner))
	}

	c := card.Card{Title: s.Title.Render(e.Title), Body: body}

	return c.Render(card.Styles{Edge: s.Edge}, w)
}

// Card is Block centered in a frame width by height cells, every line padded to width, so it can
// stand in for a list's whole body.
func Card(s skin.Skin, e Empty, width, height int) []string {
	if width < 1 || height < 1 {
		return nil
	}

	block := Block(s, e, width)
	if len(block) > height {
		block = block[:height]
	}

	out := make([]string, height)
	top := (height - len(block)) / halves

	for i := range out {
		line := ""
		if j := i - top; j >= 0 && j < len(block) {
			line = strings.Repeat(" ", max(0, (width-lipgloss.Width(block[j]))/halves)) + block[j]
		}

		out[i] = line + strings.Repeat(" ", max(0, width-lipgloss.Width(line)))
	}

	return out
}

func wrap(text string, width int) []string {
	var out []string

	for _, w := range strings.Fields(text) {
		if n := len(out); n > 0 && lipgloss.Width(out[n-1])+1+lipgloss.Width(w) <= width {
			out[n-1] += " " + w
		} else {
			out = append(out, table.Truncate(w, width))
		}
	}

	return out
}
