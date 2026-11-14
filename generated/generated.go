// Package generated answers whether a file was written by a machine, so a diff can fold
// lockfiles, vendored trees, and generated code and report their size instead of
// showing them.
package generated

import (
	"path"
	"strings"
)

// Patterns are the built-in matches. Each is a path suffix, a base name, or a directory
// name with a trailing slash matched anywhere in the path. Globs are not accepted.
var Patterns = []string{
	".min.css",
	".min.js",
	".pb.go",
	".snap",
	"Cargo.lock",
	"Gemfile.lock",
	"composer.lock",
	"go.sum",
	"node_modules/",
	"package-lock.json",
	"pnpm-lock.yaml",
	"poetry.lock",
	"uv.lock",
	"vendor/",
	"yarn.lock",
	"_generated.go",
	"_pb2.py",
	"__snapshots__/",
}

// Set is the patterns that count as generated in one repository.
type Set struct {
	suffixes []string
	dirs     []string
}

// New builds a Set from Patterns plus extra, which add to the built-ins rather than
// replacing them.
func New(extra []string) Set {
	var s Set

	for _, p := range append(append([]string{}, Patterns...), extra...) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if dir, cut := strings.CutSuffix(p, "/"); cut {
			s.dirs = append(s.dirs, dir)

			continue
		}

		s.suffixes = append(s.suffixes, p)
	}

	return s
}

// Match reports whether a path names a generated file. Matching is case-sensitive.
func (s Set) Match(p string) bool {
	base := path.Base(p)

	for _, want := range s.suffixes {
		if base == want || strings.HasSuffix(p, want) {
			return true
		}
	}

	for _, dir := range s.dirs {
		if strings.HasPrefix(p, dir+"/") || strings.Contains(p, "/"+dir+"/") {
			return true
		}
	}

	return false
}
