package palette_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/palette"
)

var update = flag.Bool("update", false, "rewrite the golden frames")

// TestFrames pins the palette over the shared vocabulary at the two sizes a tool
// opens it at most: the strip at 80x24 and the drawer at 100x30. The vocabulary
// carries "ctrl+e/shift+down", the widest label there is, so these frames are
// where a key column sized to it would cut every description.
func TestFrames(t *testing.T) {
	t.Parallel()

	km, err := keys.Merge(slices.Concat(keys.Global(), keys.DiffView()), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	es := palette.FromKeymap(km, keys.View, nil, nil)

	for _, size := range [][2]int{{80, 24}, {100, 30}} {
		for _, mode := range []palette.Mode{palette.Browse, palette.Run} {
			name := fmt.Sprintf("%s-%dx%d", map[palette.Mode]string{palette.Browse: "browse", palette.Run: "run"}[mode],
				size[0], size[1])

			t.Run(name, func(t *testing.T) {
				t.Parallel()

				m := palette.New(sk(), palette.WithClock(func() time.Time { return now }))
				m.Open(mode, es)

				// Typing finds the scroll bindings, which have alternates.
				if mode == palette.Run {
					press(m, "s", "c", "r", "o", "l", "l")
				}

				golden(t, name, drawn(t, m, size[0], size[1])+"\n")
			})
		}
	}
}

// golden compares got with testdata/<name>.golden, rewriting it under -update.
func golden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", filepath.Base(name)+".golden")

	if *update {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	if got != string(want) {
		t.Errorf("%s differs from its golden (go test ./tui/palette -update):\n%s", name, got)
	}
}
