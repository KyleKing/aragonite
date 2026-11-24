package seen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/seen"
)

const fullID = "qwounpzzkmrtsvxylnopqrstuvwxyzkk"

func writeScope(t *testing.T, dir, name, body string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func hunkRecord(hash string) string { return "[[hunk]]\nhash = '" + hash + "'\npath = 'a.txt'\n" }

func TestMigrate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		files map[string]string
		name  string
		moved bool
	}{
		{name: "one prefix moves", files: map[string]string{"qwounpzz": hunkRecord("h1")}, moved: true},
		{name: "a nine-character prefix moves", files: map[string]string{"qwounpzzk": hunkRecord("h1")}, moved: true},
		{name: "nothing to move", files: map[string]string{"other123": hunkRecord("h1")}},
		{name: "an empty directory", files: map[string]string{}},
		{
			name:  "two prefixes are ambiguous",
			files: map[string]string{"qwounpzz": hunkRecord("h1"), "qwounpzzk": hunkRecord("h2")},
		},
		{name: "the full file wins", files: map[string]string{"qwounpzz": hunkRecord("h1"), fullID: hunkRecord("h2")}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, body := range tc.files {
				writeScope(t, dir, name, body)
			}

			moved, err := seen.Migrate(dir, fullID)
			if err != nil || moved != tc.moved {
				t.Fatalf("Migrate = %v, %v; want %v", moved, err, tc.moved)
			}

			if !tc.moved {
				return
			}

			got, err := os.ReadFile(filepath.Join(dir, fullID+".toml")) //nolint:gosec // the test's own file
			if err != nil || !strings.Contains(string(got), "h1") {
				t.Errorf("the full file holds %q (%v)", got, err)
			}

			if _, err := os.Stat(filepath.Join(dir, fullID+".toml.lock")); !os.IsNotExist(err) {
				t.Errorf("the lock was left behind: %v", err)
			}
		})
	}
}

// A missing store directory has nothing to move and is no error.
func TestMigrate_MissingDirectory(t *testing.T) {
	t.Parallel()

	if moved, err := seen.Migrate(filepath.Join(t.TempDir(), "none"), fullID); moved || err != nil {
		t.Errorf("Migrate = %v, %v", moved, err)
	}
}

// Store.Migrate drops a set it loaded before the move, so the moved marks read at once.
func TestStore_MigrateReloadsTheScope(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeScope(t, dir, "qwounpzz", hunkRecord("h1"))

	st := seen.Open(dir)
	if !st.Unread(fullID, "h1") {
		t.Fatal("h1 reads as read before the migration")
	}

	moved, err := st.Migrate(fullID)
	if err != nil || !moved {
		t.Fatalf("Migrate = %v, %v", moved, err)
	}

	if st.Unread(fullID, "h1") {
		t.Error("h1 reads as unread after the migration")
	}

	if _, err := st.Migrate(""); err == nil {
		t.Error("an empty scope migrates")
	}
}
