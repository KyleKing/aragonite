package keyhint

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/table"
)

// Page draws one level of a legend as wrapped hint chips rather than a row a
// key, which is what a legend long enough to scroll needs. A Hint marked Head
// opens a group: its What is the label in the left margin and the hints under
// it wrap in the column beside it, so the names stay visible without spending
// a line each. A Hint with no Key is a line of prose, and one with Kids
// carries an ellipsis saying a press of its key opens that page.
//
// The margin is as wide as the widest group name; a page with no headings has
// none, and the chips start at the left margin itself.
func Page(s Styles, hints []Hint, width int) []string {
	margin := 0
	for _, h := range hints {
		if h.Head {
			margin = max(margin, lipgloss.Width(h.What))
		}
	}

	col := 0
	if margin > 0 {
		col = margin + len(Gutter)
	}

	room := func() int { return max(1, width-len(Indent)-col) }

	var out []string
	line, empty := "", true

	// chip appends one hint to the line being built, starting a fresh column
	// line when it would run past the width. A hint too long for the column is
	// shortened rather than wrapped: a chip broken mid-word reads worse than
	// one that ends early.
	chip := func(h Hint) {
		if len(h.Kids) > 0 {
			h.What += "…"
		}

		h.What = table.Truncate(h.What, max(1, room()-lipgloss.Width(h.Key)-3))

		one := One(s, h)
		if line == "" {
			line = Indent + strings.Repeat(" ", col)
		}

		if !empty && lipgloss.Width(line)+len(Gap)+lipgloss.Width(one) > width {
			out = append(out, line)
			line, empty = Indent+strings.Repeat(" ", col), true
		}

		if !empty {
			line += Gap
		}

		line += one
		empty = false
	}

	for _, h := range hints {
		switch {
		case h.Head:
			if line != "" {
				out = append(out, line)
			}

			line = Indent + s.Head.Render(table.Pad(h.What, margin, table.AlignRight)) + Gutter
			empty = true
		case h.Key == "":
			if line != "" {
				out = append(out, line)
				line, empty = "", true
			}

			if h.What == "" {
				out = append(out, "")
			} else {
				for _, w := range prose(h.What, room()) {
					out = append(out, Indent+strings.Repeat(" ", col)+s.Text.Render(w))
				}
			}
		default:
			chip(h)
		}
	}

	if line != "" {
		out = append(out, line)
	}

	return out
}

// prose breaks a line of prose at its word boundaries, so a sentence too long
// for the column reads on rather than ending mid-word.
func prose(what string, room int) []string {
	var out []string

	for _, w := range strings.Fields(what) {
		if n := len(out); n > 0 && lipgloss.Width(out[n-1])+1+lipgloss.Width(w) <= room {
			out[n-1] += " " + w
		} else {
			out = append(out, w)
		}
	}

	// A single word can still be wider than the column.
	for i, line := range out {
		out[i] = table.Truncate(line, room)
	}

	return out
}

// At walks a path of keys into a legend's nested pages and answers the page it
// reaches, so a caller can keep the path it took rather than a stack of pages.
// The second return is false when a key on the path opens nothing, which a
// held path can dead-end into once the hints it names are rebuilt.
func At(root []Hint, path []string) ([]Hint, bool) {
	page := root

	for _, k := range path {
		next := kids(page, k)
		if next == nil {
			return nil, false
		}

		page = next
	}

	return page, true
}

// kids is the page under the hint k names, or nil when no hint with that key
// opens one. A hint with no Kids is a leaf and opens nothing.
func kids(page []Hint, k string) []Hint {
	for _, h := range page {
		if h.Key == k && len(h.Kids) > 0 {
			return h.Kids
		}
	}

	return nil
}
