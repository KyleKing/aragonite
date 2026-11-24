package framecheck

import (
	"fmt"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kyleking/aragonite/tui/glyph"
)

// SourceOption adjusts what Source accepts.
type SourceOption func(*sourceOptions)

type sourceOptions struct {
	skip    []string
	private []string
}

// SkipFiles leaves out files whose slash path, relative to the root, matches
// one of the patterns (path.Match syntax, or a plain suffix): a table of
// content stand-ins, or a test fixture that lives beside the code.
func SkipFiles(patterns ...string) SourceOption {
	return func(o *sourceOptions) { o.skip = append(o.skip, patterns...) }
}

// PrivateIn accepts private-use glyphs in the files matching patterns, which is
// where a Nerd icon table lives. Everywhere else one is a fault.
func PrivateIn(patterns ...string) SourceOption {
	return func(o *sourceOptions) { o.private = append(o.private, patterns...) }
}

// Source holds every string and rune literal in the Go files under root (tests
// and testdata left out) to the chrome glyph policy: no glyph whose width
// terminals disagree on, and no wide glyph, since a screen's own marks must
// line up in every terminal. It reports each one through t with its file, line,
// and class, and a tool calls it once from a test over its own module.
func Source(t TB, root string, opts ...SourceOption) {
	t.Helper()

	o := sourceOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relating %s to %s: %w", path, root, err)
		}

		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if rel != "." && skippedDir(d.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || matches(rel, o.skip) {
			return nil
		}

		for _, f := range scanFile(path, rel, matches(rel, o.private)) {
			t.Errorf("%s", f)
		}

		return nil
	})
	if err != nil {
		t.Errorf("scanning %s: %v", root, err)
	}
}

func skippedDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "testdata" || name == "vendor"
}

func matches(rel string, patterns []string) bool {
	for _, p := range patterns {
		if ok, err := filepath.Match(p, rel); err == nil && ok || strings.HasSuffix(rel, p) {
			return true
		}
	}

	return false
}

func scanFile(path, rel string, private bool) []string {
	src, err := os.ReadFile(path) //nolint:gosec // a path the walk found under the root asked for
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", rel, err)}
	}

	var (
		s   scanner.Scanner
		out []string
	)

	fset := token.NewFileSet()
	s.Init(fset.AddFile(rel, -1, len(src)), src, nil, 0)

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}

		if tok != token.STRING && tok != token.CHAR {
			continue
		}

		text, err := strconv.Unquote(lit)
		if err != nil {
			continue
		}

		glyph.Each(text, func(cluster string, _ int, c glyph.Class) bool {
			if c == glyph.Private && private {
				return true
			}

			out = append(out, fmt.Sprintf("%s: %q (U+%04X) is %s; chrome uses width-stable glyphs only",
				fset.Position(pos), cluster, first(cluster), c))

			return true
		})
	}
}
