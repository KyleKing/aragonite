package hermetic

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// RequireEveryPackage fails tb for each directory of the module at root that holds tests and no
// test file calling hermetic.Main or hermetic.Run, so a package added later cannot reach the real
// home unnoticed. Call it from one test, with the module's root directory relative to that test's
// package (filepath.Join("..", "..") from internal/x).
func RequireEveryPackage(tb testing.TB, root string) {
	tb.Helper()

	module, err := filepath.Abs(root)
	if err != nil {
		tb.Fatal(err)
	}

	missing, err := Unguarded(module)
	if err != nil {
		tb.Fatal(err)
	}

	for _, rel := range missing {
		tb.Errorf("%s has tests and no TestMain calling hermetic.Main or hermetic.Run", rel)
	}
}

// Unguarded is every directory under module, relative to it and sorted, holding a _test.go file
// while none of its test files calls hermetic.Main( or hermetic.Run(. Hidden directories,
// testdata, vendor, and nested modules are not walked.
func Unguarded(module string) ([]string, error) {
	guarded := map[string]bool{}

	err := filepath.WalkDir(module, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && path != module && skipDir(path, d.Name()):
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, "_test.go"):
			return nil
		}

		data, err := os.ReadFile(path) //nolint:gosec // a test file of the module
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		dir, text := filepath.Dir(path), string(data)
		guarded[dir] = guarded[dir] ||
			strings.Contains(text, "hermetic.Main(") || strings.Contains(text, "hermetic.Run(")

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", module, err)
	}

	var out []string

	for dir, ok := range guarded {
		if !ok {
			rel, err := filepath.Rel(module, dir)
			if err != nil {
				return nil, fmt.Errorf("relating %s to %s: %w", dir, module, err)
			}

			out = append(out, rel)
		}
	}

	slices.Sort(out)

	return out, nil
}

func skipDir(path, name string) bool {
	if strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor" {
		return true
	}

	_, err := os.Stat(filepath.Join(path, "go.mod"))

	return err == nil
}
