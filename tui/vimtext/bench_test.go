package vimtext_test

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/aragonite/tui/vimtext"
)

// twoHundred is a 200-line buffer of prose with some wide text in it.
func twoHundred() string {
	lines := make([]string, 200)
	for i := range lines {
		//nolint:gosmopolitan // CJK is the point: wide characters cost a layout of their own
		lines[i] = "line " + strconv.Itoa(i) + ": the quick brown fox jumps over the lazy dog, 日本語 (naïve) " +
			strings.Repeat("x", i%40)
	}

	return strings.Join(lines, "\n")
}

// BenchmarkKeystroke is one key and the frame it redraws, on a 200-line buffer
// at 80 columns: the cost a person pays per key.
func BenchmarkKeystroke(b *testing.B) {
	for _, c := range []struct {
		name, setup string
		cycle       []string
	}{
		{"insert-char", "", []string{"a", "<BS>"}},
		{"newline", "", []string{"<C-j>", "<BS>"}},
		{"normal-w", "<Esc>gg", []string{"w"}},
		{"normal-j-k", "<Esc>50G", []string{"j", "k"}},
		{"G-gg", "<Esc>", []string{"G", "gg"}},
		{"x-u", "<Esc>gg", []string{"x", "u"}},
		{"dd-u", "<Esc>100G", []string{"d", "d", "u"}},
		{"ciw-dot-u", "<Esc>gg", []string{"c", "i", "w", "z", "<Esc>", "w", ".", "u", "u"}},
		{"visual-j", "<Esc>100Gv", []string{"j", "k"}},
		{"dap-u", "<Esc>100G", []string{"d", "a", "p", "u"}},
	} {
		b.Run(c.name, func(b *testing.B) {
			f := vimtext.New(twoHundred(), vimtext.WithHeight(1, 20))
			press(b, f, c.setup)

			cycle := make([]tea.KeyPressMsg, 0, len(c.cycle))
			for _, k := range c.cycle {
				cycle = append(cycle, keys(b, k)...)
			}

			f.View(80)
			b.ReportAllocs()
			b.ResetTimer()

			for i := range b.N {
				f.Update(cycle[i%len(cycle)])
				_ = f.View(80)
			}
		})
	}
}

// BenchmarkView is a frame with nothing changed, the cost of a redraw for a
// message that is not a key.
func BenchmarkView(b *testing.B) {
	f := vimtext.New(twoHundred(), vimtext.WithHeight(1, 20))
	press(b, f, "<Esc>100G")
	b.ReportAllocs()

	for range b.N {
		_ = f.View(80)
	}
}
