package vimtext_test

import (
	"testing"

	"github.com/kyleking/aragonite/tui/vimtext"
)

// play types c.keys into a field holding c.from, from normal mode at the
// marked cursor, and answers what it holds with the cursor marked, and its
// mode.
func play(t *testing.T, c keyCase) (string, string) {
	t.Helper()

	text, line, col := marked(t, c.from)
	f := vimtext.New(text)
	f.Place(line, col, vimtext.Normal)
	press(t, f, c.keys)

	l, cl := f.Cursor()

	return mark(f.Value(), l, cl), f.Mode().String()
}

// TestKeys is every motion, operator, text object, count, undo, redo, and
// repeat as a key sequence and the text, cursor, and mode it leaves.
func TestKeys(t *testing.T) {
	t.Parallel()

	for _, c := range keyCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, mode := play(t, c)

			ends := c.ends
			if ends == "" {
				ends = "NORMAL"
			}

			if got != c.want || mode != ends {
				t.Errorf("%q on %q:\n got %q %s\nwant %q %s", c.keys, c.from, got, mode, c.want, ends)
			}
		})
	}
}
