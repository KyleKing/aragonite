package seen

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const scopeExt = ".toml"

// Migrate moves the marks a store kept under a prefix of a scope into the scope's own file,
// for a tool that once scoped by a shortened id. Older jojo scoped by jj's
// change_id.shortest(8), which grows once another change comes to share its prefix, so the
// file a change's marks were filed under is named by a prefix of its full id, and every mark
// on it silently vanished when the prefix grew.
//
// When dir has no file for full and exactly one file whose name is a proper prefix of full,
// that file's marks become full's: renamed, or merged into a full file another process wrote
// meanwhile. None, or two candidates, leaves nothing safe to move. It takes the lock Save
// takes on full's file, so a concurrent save of that scope waits rather than losing a mark.
// The full name is a file name as Store writes it, which for a jj change id (lowercase
// letters) is the id itself. It reports whether it moved anything.
func Migrate(dir, full string) (bool, error) {
	target := filepath.Join(dir, full+scopeExt)
	if _, err := os.Stat(target); err == nil || !errors.Is(err, fs.ErrNotExist) {
		// A full file already there, or one that cannot be read, is not ours to move into.
		return false, nil
	}

	old, err := prefixFile(dir, full)
	if old == "" || err != nil {
		return false, err
	}

	unlock, err := lock(target)
	if err != nil {
		return false, err
	}

	moved, err := move(old, target)

	return moved, errors.Join(err, unlock())
}

// Migrate is the package Migrate on the store's directory for scope, and drops the scope's cached
// Set when it moved anything, so the next ask reads the marks it moved.
func (st *Store) Migrate(scope string) (bool, error) {
	path, err := st.Path(scope)
	if err != nil {
		return false, err
	}

	moved, err := Migrate(st.dir, strings.TrimSuffix(filepath.Base(path), scopeExt))
	if moved {
		st.Reload(scope)
	}

	return moved, err
}

// prefixFile is the one scope file in dir whose name is a proper prefix of full, "" when there is
// none or more than one.
func prefixFile(dir, full string) (string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("listing %s: %w", dir, err)
	}

	var found []string

	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), scopeExt)
		if ok && name != "" && !e.IsDir() && name != full && strings.HasPrefix(full, name) {
			found = append(found, filepath.Join(dir, e.Name()))
		}
	}

	if len(found) != 1 {
		return "", nil
	}

	return found[0], nil
}

// move renames old onto target, or merges it into a target written since Migrate looked.
func move(old, target string) (bool, error) {
	if _, err := os.Stat(target); errors.Is(err, fs.ErrNotExist) {
		if err := os.Rename(old, target); err != nil {
			return false, fmt.Errorf("moving %s to %s: %w", old, target, err)
		}

		return true, nil
	}

	merged, err := mergeFiles(target, old)
	if err != nil {
		return false, err
	}

	if err := replace(target, merged); err != nil {
		return false, err
	}

	if err := os.Remove(old); err != nil {
		return false, fmt.Errorf("removing %s: %w", old, err)
	}

	return true, nil
}

// mergeFiles is the union of the hunks scope files hold, each hash once, in the order first seen.
func mergeFiles(paths ...string) ([]byte, error) {
	var out file

	have := map[string]bool{}

	for _, p := range paths {
		raw, err := os.ReadFile(p) //nolint:gosec // a scope file of the store being migrated
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}

		var f file
		if err := toml.NewDecoder(bytes.NewReader(raw)).Decode(&f); err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}

		for _, r := range f.Hunk {
			if !have[r.Hash] {
				have[r.Hash] = true
				out.Hunk = append(out.Hunk, r)
			}
		}
	}

	body, err := toml.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encoding the merged marks: %w", err)
	}

	return body, nil
}
