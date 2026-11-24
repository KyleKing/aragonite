package hermetic

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// stamp is what a snapshot keeps of one path: enough to tell it was rewritten.
type stamp struct {
	mod  time.Time
	size int64
	mode fs.FileMode
}

// Snapshot is every file and directory under some paths, by path.
type Snapshot map[string]stamp

// Take snapshots paths, each a file or a directory walked whole. A missing path is recorded as
// missing, so creating it reads as a change.
func Take(paths ...string) Snapshot {
	s := Snapshot{}

	for _, root := range paths {
		// The walk never fails: an unreadable entry is skipped and the rest recorded.
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error { //nolint:errcheck // see above
			if err != nil {
				return nil //nolint:nilerr // a missing or unreadable path is simply not recorded
			}

			info, err := d.Info()
			if err != nil {
				return nil //nolint:nilerr // vanished between the listing and the stat
			}

			s[path] = stamp{mod: info.ModTime(), size: info.Size(), mode: info.Mode()}

			return nil
		})
	}

	return s
}

// Changed lists every path added, removed, or rewritten between s and after, sorted.
func (s Snapshot) Changed(after Snapshot) []string {
	var out []string

	for path, was := range s {
		if now, ok := after[path]; !ok || now != was {
			out = append(out, path)
		}
	}

	for path := range after {
		if _, ok := s[path]; !ok {
			out = append(out, path)
		}
	}

	slices.Sort(out)

	return out
}

// Leaks is what changed between before and after that this run did. Under jj's repos/ an entry is
// the run's only when one of its files names root (as given or with symlinks resolved), since jj
// records each repository by its path and every repository a test makes is under root; another
// program on the machine opening its own repository meanwhile is not a leak. A change anywhere
// else is.
func Leaks(before, after Snapshot, jjDir, root string) []string {
	repos := filepath.Join(jjDir, "repos")
	roots := [][]byte{[]byte(root)}

	if resolved, err := filepath.EvalSymlinks(root); err == nil && resolved != root {
		roots = append(roots, []byte(resolved))
	}

	ours := map[string]bool{}

	var out []string

	for _, path := range before.Changed(after) {
		rel, err := filepath.Rel(repos, path)

		switch {
		case err != nil || strings.HasPrefix(rel, ".."):
			out = append(out, path)

			continue
		case rel == ".":
			// The directory itself changes with any entry added under it; the entry decides.
			continue
		}

		entry := filepath.Join(repos, strings.Split(rel, string(filepath.Separator))[0])
		if _, seen := ours[entry]; !seen {
			ours[entry] = names(entry, roots)
		}

		if ours[entry] {
			out = append(out, path)
		}
	}

	return out
}

// names reports whether any file in dir holds one of roots.
func names(dir string, roots [][]byte) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // jj's own record of a repository
		if err != nil {
			continue
		}

		for _, root := range roots {
			if bytes.Contains(data, root) {
				return true
			}
		}
	}

	return false
}
