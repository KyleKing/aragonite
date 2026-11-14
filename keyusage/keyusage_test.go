package keyusage_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/keyusage"
)

func keymap() keyusage.Keymap {
	return keyusage.Keymap{
		"review": {
			{Keys: []string{"j", "down"}, What: "line down"},
			{Keys: []string{"G"}, What: "bottom"},
			{Keys: []string{"m"}, What: "a state", Kids: []keyusage.Binding{
				{Keys: []string{"x"}, What: "skip"},
				{Keys: []string{"r"}, What: "ready"},
			}},
			{Keys: []string{"r"}, What: "ready verdict"},
			{Keys: []string{"F"}, What: "fixme"},
		},
		"queue": {
			{Keys: []string{"r"}, What: "reply"},
		},
	}
}

func record(t *testing.T, dir string, start time.Time, keys [][2]string) {
	t.Helper()

	now := start
	r := keyusage.Open("app", keyusage.WithDir(dir), keyusage.WithHost("h"),
		keyusage.WithClock(func() time.Time { now = now.Add(time.Minute); return now }))

	for _, k := range keys {
		r.Key(k[0], k[1])
	}

	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	start := time.Date(2026, 10, 31, 23, 58, 0, 0, time.UTC)
	record(t, dir, start, [][2]string{
		{"review", "down"},
		{"review", "j"},
		{"review", "G"},
		{"review", "m"},
		{"review", "x"},
		{"review", "m"},
		{"review", "q"},
		{"review", "s"},
		{"queue", "r"},
		{"elsewhere", "z"},
	})

	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(files) != 2 {
		t.Fatalf("want a file per month, got %v (%v)", files, err)
	}

	km := keymap()

	u, err := keyusage.Load(dir, km)
	if err != nil {
		t.Fatal(err)
	}

	checkCounts(t, u)
	checkSignals(t, u, km, start)
	checkReport(t, u, km, start)
}

func checkCounts(t *testing.T, u keyusage.Usage) {
	t.Helper()

	for _, c := range []struct {
		ctx, path string
		want      int
	}{
		{"review", "j", 0},
		{"review", "G", 1},
		{"review", "m x", 1},
		{"review", "m r", 0},
		{"review", "F", 0},
		{"queue", "r", 1},
	} {
		if got := u.Stat(c.ctx, c.path).Count; got != c.want {
			t.Errorf("%s %q: got %d, want %d", c.ctx, c.path, got, c.want)
		}
	}
}

func checkSignals(t *testing.T, u keyusage.Usage, km keyusage.Keymap, start time.Time) {
	t.Helper()

	misses := map[string]int{}
	for _, r := range u.Misses() {
		misses[r.Path] = r.Count
	}

	if misses["m q"] != 1 || misses["s"] != 1 {
		t.Errorf("misses: %v", misses)
	}

	if u.Unmapped["elsewhere"] != 1 {
		t.Errorf("unmapped: %v", u.Unmapped)
	}

	for _, c := range []struct {
		at   time.Time
		path string
		want int
	}{
		{start, "F", 3},
		{start.AddDate(0, 0, 1), "m x", 0},
		{start.AddDate(0, 0, 8), "m x", 1},
		{start.AddDate(0, 0, 15), "m x", 2},
		{start.AddDate(0, 0, 29), "m x", 3},
	} {
		if got := u.Level("review", c.path, c.at); got != c.want {
			t.Errorf("level %q at %s: got %d, want %d", c.path, c.at, got, c.want)
		}
	}

	over := u.Overloads(km)
	if len(over) != 1 || over[0].Key != "r" || len(over[0].Uses) != 2 {
		t.Errorf("overloads: %+v", over)
	}
}

func checkReport(t *testing.T, u keyusage.Usage, km keyusage.Keymap, start time.Time) {
	t.Helper()

	var out bytes.Buffer
	if err := keyusage.Write(&out, u, km, start.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"never used (3)", "m r", "pressed with nothing bound", "m q", "one key, different meanings (1)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, out.String())
		}
	}
}

func TestOpenOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(keyusage.EnvVar("some-app"), "")

	if r := keyusage.Open("some-app"); r != nil {
		t.Fatal("no ~/Sync and no override should record nothing")
	}

	r := (*keyusage.Recorder)(nil)
	r.Key("x", "y")

	dir := t.TempDir()
	t.Setenv(keyusage.EnvVar("some-app"), dir)

	keyusage.Open("some-app").Key("ctx", "F")

	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Fatalf("override dir should hold one log, got %d (%v)", len(entries), err)
	}
}

func TestInvocations(t *testing.T) {
	t.Parallel()

	root := keyusage.Command{Name: "tool", Flags: []string{"--version"}, Subs: []keyusage.Command{
		{Name: "pr", Subs: []keyusage.Command{{Name: "view", Flags: []string{"--web", "--repo"}}}},
		{Name: "keys"},
	}}

	dir := t.TempDir()
	r := keyusage.Open("tool", keyusage.WithDir(dir), keyusage.WithHost("h"))

	for _, args := range [][]string{
		{"pr", "view", "123", "--repo=secret/name", "--web"},
		{"pr", "view", "--repo", "secret/name", "keys"},
		{"--nope", "secret-path"},
	} {
		r.Invoke(root, args)
	}

	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	log := filepath.Join(dir, "h-"+time.Now().Format("2006-01")+".jsonl")

	raw, err := os.ReadFile(log) //nolint:gosec // the test's temp dir
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "123") {
		t.Fatalf("a positional argument or flag value was recorded:\n%s", raw)
	}

	km := keyusage.Keymap{keyusage.CLI: keyusage.Commands(root)}

	u, err := keyusage.Load(dir, km)
	if err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]int{
		"tool pr view": 2, "tool pr view --repo": 2, "tool pr view --web": 1, "tool": 1, "tool keys": 0,
	} {
		if got := u.Stat(keyusage.CLI, path).Count; got != want {
			t.Errorf("%q: got %d, want %d", path, got, want)
		}
	}

	if m := u.Misses(); len(m) != 1 || m[0].Path != "tool --nope" {
		t.Errorf("misses: %+v", m)
	}
}
