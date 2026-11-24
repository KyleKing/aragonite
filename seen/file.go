package seen

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	dirPerm  = 0o750
	filePerm = 0o600
)

// record is one entry on disk. The hash is what matters; the path and the header are there so a
// person reading the file can tell what it refers to.
type record struct {
	Hash   string `toml:"hash"`
	Path   string `toml:"path"`
	Header string `toml:"header,omitempty"`
}

// file is the on-disk shape, a TOML array of [[hunk]] tables. It is what second-look wrote before
// this package existed, and it must keep reading that.
type file struct {
	Hunk []record `toml:"hunk"`
}

// Load reads the set at path. A missing file is an empty set, not an error: a review nobody has
// read yet is the normal first case.
func Load(path string) (*Set, error) {
	ids, err := readIDs(path)
	if err != nil {
		return nil, err
	}

	s := New()
	s.ids = ids

	return s, nil
}

func readIDs(path string) (map[ID]bool, error) {
	ids := map[ID]bool{}

	raw, err := os.ReadFile(path) //nolint:gosec // a path the caller chose for its own state
	if errors.Is(err, fs.ErrNotExist) {
		return ids, nil
	}

	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var f file

	dec := toml.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	for _, r := range f.Hunk {
		ids[ID(r.Hash)] = true
	}

	return ids, nil
}

// Save writes the set to path, keeping only the hunks live still carries. A mark for a hunk that
// no longer exists is dead weight, and a file that grows forever is one nobody can read.
//
// It is safe against a second process saving the same path. Under a lock file beside path it
// rereads what is there, applies only what this Set changed since it was loaded or last saved,
// and replaces the file through a rename, so a crash never leaves half a file and a mark the
// other process saved meanwhile survives. Afterwards the Set holds what was written.
func Save(path string, s *Set, live []Ref) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}

	unlock, err := lock(path)
	if err != nil {
		return err
	}

	defer func() { err = errors.Join(err, unlock()) }()

	disk, err := readIDs(path)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, read := range s.changed {
		if read {
			disk[id] = true
		} else {
			delete(disk, id)
		}
	}

	var (
		f    file
		kept = make(map[ID]bool, len(live))
	)

	for _, r := range live {
		if !disk[r.ID] || kept[r.ID] {
			continue
		}

		kept[r.ID] = true
		f.Hunk = append(f.Hunk, record{Hash: string(r.ID), Path: r.Path})
	}

	body, err := toml.Marshal(f)
	if err != nil {
		return fmt.Errorf("encoding the read hunks: %w", err)
	}

	if err := replace(path, body); err != nil {
		return err
	}

	s.ids = kept
	s.changed = map[ID]bool{}

	return nil
}

// replace writes body to a temporary file beside path and renames it into place.
func replace(path string, body []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	name := tmp.Name()

	_, err = tmp.Write(body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}

	if err == nil {
		err = os.Chmod(name, filePerm)
	}

	if err == nil {
		err = os.Rename(name, path)
	}

	if err != nil {
		if rerr := os.Remove(name); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
			err = errors.Join(err, rerr)
		}

		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
