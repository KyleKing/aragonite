package empty_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/empty"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

func sample() empty.Empty {
	return empty.Empty{
		Title: "No changes in wkwyyxvq",
		Why:   "The working copy matches its parent. The parent has 3 files: open it with -r @-.",
		Hints: []keyhint.Hint{{Key: "ctrl+l", What: "reload"}, {Key: "q", What: "quit"}},
	}
}

func TestCard_CentersAndKeepsTheFrame(t *testing.T) {
	t.Parallel()

	s := skin.New(theme.Macchiato(), theme.Macchiato().Mauve)

	for _, size := range [][2]int{{100, 30}, {80, 24}, {40, 12}, {24, 6}} {
		w, h := size[0], size[1]

		out := empty.Card(s, sample(), w, h)
		if len(out) != h {
			t.Fatalf("%dx%d: %d lines", w, h, len(out))
		}

		for i, l := range out {
			if got := ansi.StringWidth(l); got != w {
				t.Errorf("%dx%d line %d is %d wide", w, h, i, got)
			}
		}

		plain := ansi.Strip(strings.Join(out, "\n"))
		if !strings.Contains(plain, "No changes") {
			t.Errorf("%dx%d lost the title:\n%s", w, h, plain)
		}

		if h >= 12 && !strings.Contains(plain, "ctrl+l reload") {
			t.Errorf("%dx%d lost the keys:\n%s", w, h, plain)
		}
	}
}

func TestBlock_WidthBounds(t *testing.T) {
	t.Parallel()

	s := skin.New(theme.Macchiato(), theme.Macchiato().Mauve)

	if w := ansi.StringWidth(empty.Block(s, sample(), 200)[0]); w > empty.MaxWidth {
		t.Errorf("block is %d wide on a wide frame, past MaxWidth", w)
	}

	short := empty.Empty{Title: "No reviews", Hints: []keyhint.Hint{{Key: "tab", What: "other queues"}}}
	if out := empty.Block(s, short, 80); len(out) != 3 {
		t.Errorf("a card with no reason is %d rows, want border, keys, border", len(out))
	}

	if empty.Card(s, sample(), 0, 10) != nil {
		t.Error("a zero-width frame drew")
	}
}

func BenchmarkCard(b *testing.B) {
	s := skin.New(theme.Macchiato(), theme.Macchiato().Mauve)

	for b.Loop() {
		empty.Card(s, sample(), 100, 30)
	}
}
