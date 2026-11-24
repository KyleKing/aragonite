package seen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A full file another process wrote between Migrate's look and its lock is merged into, not lost.
func TestMove_MergesIntoAFullFileWrittenMeanwhile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec := func(h string) string { return "[[hunk]]\nhash = '" + h + "'\npath = 'a.txt'\n" }
	short, full := filepath.Join(dir, "qwounpzz.toml"), filepath.Join(dir, "qwounpzzkmrt.toml")

	for path, body := range map[string]string{short: rec("h1") + rec("h2"), full: rec("h2") + rec("h3")} {
		if err := os.WriteFile(path, []byte(body), filePerm); err != nil {
			t.Fatal(err)
		}
	}

	moved, err := move(short, full)
	if err != nil || !moved {
		t.Fatalf("move = %v, %v", moved, err)
	}

	got, err := os.ReadFile(full) //nolint:gosec // the test's own file
	if err != nil {
		t.Fatal(err)
	}

	for _, h := range []string{"h1", "h2", "h3"} {
		if strings.Count(string(got), "'"+h+"'")+strings.Count(string(got), `"`+h+`"`) != 1 {
			t.Errorf("the merged file holds %s other than once:\n%s", h, got)
		}
	}

	if _, err := os.Stat(short); !os.IsNotExist(err) {
		t.Errorf("the short file is still there: %v", err)
	}

	if _, err := Load(full); err != nil {
		t.Errorf("the merged file does not load: %v", err)
	}
}
