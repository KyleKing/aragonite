package keyusage_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/keyusage"
)

func TestActions_RoundTripBesideKeys(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	start := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	now := start
	r := keyusage.Open("app", keyusage.WithDir(dir), keyusage.WithHost("h"),
		keyusage.WithClock(func() time.Time { now = now.Add(time.Minute); return now }))

	r.Key("review", "G")
	r.Action("review", "bottom", "G")
	r.Action("review", "bottom", "")
	r.Action("queue", "bottom", "G")
	r.Gated("review", "resolve")
	r.Gated("review", "resolve")

	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	u, err := keyusage.Load(dir, keymap())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		got   keyusage.Stat
		count int
	}{
		{name: "key still counted once", got: u.Stat("review", "G"), count: 1},
		{name: "action in context", got: u.ActionStat("review", "bottom"), count: 2},
		{name: "action across contexts", got: u.ActionStat(keyusage.AnyContext, "bottom"), count: 3},
		{name: "gated", got: u.GatedStat("review", "resolve"), count: 2},
		{name: "never", got: u.ActionStat("review", "top"), count: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.got.Count != tc.count {
				t.Errorf("count = %d, want %d", tc.got.Count, tc.count)
			}
		})
	}

	if got := u.Actions("review"); got.Count("bottom") != 2 || got.Last("bottom").IsZero() {
		t.Errorf("Actions(review) bottom = %d at %v", got.Count("bottom"), got.Last("bottom"))
	}

	if got := u.Unused("review", []string{"top", "bottom", "quit"}); !slices.Equal(got, []string{"top", "quit"}) {
		t.Errorf("Unused = %v", got)
	}

	var out bytes.Buffer
	if err := keyusage.Write(&out, u, keymap(), start); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "pressed where it could not run (1)") {
		t.Errorf("report is missing the gated section:\n%s", out.String())
	}
}

func TestActions_OldReaderShapeStillParses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lines := `{"t":"2026-11-01T09:00:00Z","s":"review","e":"key","w":"G"}
{"t":"2026-11-01T09:01:00Z","s":"review","e":"act","w":"G"}
{"t":"2026-11-01T09:02:00Z","s":"review","e":"act","a":"bottom"}
`

	if err := os.WriteFile(filepath.Join(dir, "h-2026-11.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	u, err := keyusage.Load(dir, keymap())
	if err != nil {
		t.Fatal(err)
	}

	if u.Stat("review", "G").Count != 1 || u.ActionStat("review", "bottom").Count != 1 || u.Skipped != 1 {
		t.Errorf("G=%d bottom=%d skipped=%d, want 1 1 1 (an act line with no action is skipped)",
			u.Stat("review", "G").Count, u.ActionStat("review", "bottom").Count, u.Skipped)
	}
}

func BenchmarkLoadWithActions(b *testing.B) {
	dir := b.TempDir()
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	r := keyusage.Open("app", keyusage.WithDir(dir), keyusage.WithHost("h"),
		keyusage.WithClock(func() time.Time { now = now.Add(time.Second); return now }))

	for range 2000 {
		r.Key("review", "G")
		r.Action("review", "bottom", "G")
	}

	if err := r.Close(); err != nil {
		b.Fatal(err)
	}

	km := keymap()

	for b.Loop() {
		if _, err := keyusage.Load(dir, km); err != nil {
			b.Fatal(err)
		}
	}
}
