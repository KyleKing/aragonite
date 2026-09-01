// Package keyhint renders the keys a screen offers, in one shape everywhere:
// the letter that presses a thing bracketed inside the word for it, and
// bracketed in front where the word does not carry the letter. So `[p]ost one`
// and `[tab] switch`, never a column of keys beside a column of descriptions.
//
// The shape comes from mini.which-key: a reader who has seen the word once
// knows the key without a legend, and the same brackets caption the second key
// of a chord while it is waiting to be pressed.
//
// It knows nothing about bindings. A caller passes what its own keymap says,
// so a rebound key changes the hint without this package hearing about it.
package keyhint

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/table"
)

// Hint is one key and the thing it does.
type Hint struct {
	Key  string
	What string
}

// Styles are the two faces a hint is drawn with, passed in so the package never
// reaches for an application's palette.
type Styles struct {
	// Key draws the bracketed letter.
	Key lipgloss.Style
	// Text draws everything around it.
	Text lipgloss.Style
}

// Gap separates two hints on one line. It is two spaces because one reads as a
// word break inside a hint that carries one.
const Gap = "  "

// Line renders hints along a single line, which is what a footer holds.
func Line(s Styles, hints []Hint) string {
	out := make([]string, 0, len(hints))
	for _, h := range hints {
		out = append(out, One(s, h))
	}

	return strings.Join(out, Gap)
}

// One renders a single hint. A single-character key is bracketed where it
// appears in the word, preferring the start of a word, and the bracket carries
// the key's own case, so a shifted binding reads as `[S]ubmit`.
func One(s Styles, h Hint) string {
	at := index(h.Key, h.What)
	if at < 0 {
		return s.Key.Render("["+h.Key+"]") + " " + s.Text.Render(h.What)
	}

	// The letter matched in the word is not the key's own byte for a shifted or
	// accented binding, so its width is measured rather than assumed.
	_, size := utf8.DecodeRuneInString(h.What[at:])

	return s.Text.Render(h.What[:at]) +
		s.Key.Render("["+h.Key+"]") +
		s.Text.Render(h.What[at+size:])
}

// index is where the key sits in the word, or -1 where it does not. A word
// start wins over a letter in the middle, since that is the one a reader finds
// without looking.
func index(key, what string) int {
	first, size := utf8.DecodeRuneInString(key)
	if size != len(key) || what == "" {
		return -1
	}

	want := unicode.ToLower(first)
	inside := -1

	for i, r := range what {
		if unicode.ToLower(r) != want {
			continue
		}

		if i == 0 || what[i-1] == ' ' {
			return i
		}

		if inside < 0 {
			inside = i
		}
	}

	return inside
}

// Indent is the left margin a help legend is drawn in.
const Indent = "  "

// Gutter separates the key column from what the key does.
const Gutter = "  "

// Help renders a key legend: the keys right-aligned in a column as wide as the
// widest one, then what each does. Right-aligning them gives the reader one
// edge to scan rather than two, and a chord as long as `S then S / a / r / c`
// widens the column instead of running into the description beside it.
//
// A Hint carrying no Key is a line of prose, drawn in the description column,
// which is how a legend says the sentence or two the keys cannot.
func Help(s Styles, hints []Hint, width int) []string {
	keys := 0
	for _, h := range hints {
		keys = max(keys, lipgloss.Width(h.Key))
	}

	room := max(1, width-len(Indent)-keys-len(Gutter))

	out := make([]string, 0, len(hints))

	for _, h := range hints {
		if h.Key == "" {
			out = append(out, Indent+strings.Repeat(" ", keys+len(Gutter))+
				s.Text.Render(table.Truncate(h.What, room)))

			continue
		}

		out = append(out, Indent+
			s.Key.Render(table.Pad(h.Key, keys, table.AlignRight))+
			Gutter+s.Text.Render(table.Truncate(h.What, room)))
	}

	return out
}
