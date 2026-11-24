package seen

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrScope is returned for a scope that cannot name a file: empty, "." or "..".
var ErrScope = errors.New("a read-mark scope needs a name")

// Store keeps one Set per scope, each in its own file under one directory. The scope is the
// caller's: second-look uses a pull request ("pr-12") or "local", and jojo a jj change id, so a
// change's marks survive every rebase that leaves its hunks' text alone. Sets are loaded on first
// use and kept, and Save merges with what other processes saved, so two tools sharing a directory
// see each other's marks from their next save or their next Store.
//
// The read methods answer as for an empty store on a nil Store, which is a caller's "remember
// nothing" default.
type Store struct {
	sets map[string]*Set
	dir  string
	mu   sync.Mutex
}

// Open is the store kept under dir. Nothing is read or created until a scope is asked for.
func Open(dir string) *Store { return &Store{dir: dir, sets: map[string]*Set{}} }

// StateDir is where app keeps local state, with parts joined under it:
// $XDG_STATE_HOME/<app>/<parts...>, else ~/.local/state/<app>/<parts...>. Read marks are state
// rather than cache (losing them costs the reader work) and rather than config (nobody edits
// them), which is why they go here.
func StateDir(app string, parts ...string) (string, error) {
	base := os.Getenv("XDG_STATE_HOME")

	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding the state directory: %w", err)
		}

		base = filepath.Join(home, ".local", "state")
	}

	return filepath.Join(append([]string{base, app}, parts...)...), nil
}

// RepoKey names a repository for a directory under StateDir: its base name, for a person reading
// the directory, and a short digest of the cleaned path, so two clones both named "app" keep
// separate marks. Two tools that should share marks for one repository must pass the same path:
// for jj, the repository's store rather than a workspace root, since every workspace of a
// repository shares its change ids.
func RepoKey(root string) string {
	clean := filepath.Clean(root)
	sum := sha256.Sum256([]byte(clean))

	return escape(filepath.Base(clean)) + "-" + hex.EncodeToString(sum[:6])
}

// Dir is the directory the store keeps its files in.
func (st *Store) Dir() string { return st.dir }

// Path is the file a scope's marks are kept in: <dir>/<scope>.toml, with every byte outside
// [A-Za-z0-9._-] percent-escaped so any scope names exactly one file in dir.
func (st *Store) Path(scope string) (string, error) {
	if scope == "" || scope == "." || scope == ".." {
		return "", fmt.Errorf("%q: %w", scope, ErrScope)
	}

	return filepath.Join(st.dir, escape(scope)+".toml"), nil
}

// Set is a scope's marks, read from disk the first time it is asked for. The same *Set comes back
// on every later call, so a caller may mark through it and then Save the scope.
func (st *Store) Set(scope string) (*Set, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if s, ok := st.sets[scope]; ok {
		return s, nil
	}

	path, err := st.Path(scope)
	if err != nil {
		return nil, err
	}

	s, err := Load(path)
	if err != nil {
		return nil, err
	}

	st.sets[scope] = s

	return s, nil
}

// Unread reports whether a hunk of a scope has not been read. A scope whose file cannot be read
// answers unread for every hunk, since showing a hunk again is the safe failure. Its signature is
// the one a consumer's read-state seam declares with plain strings (jojo's model.ReadState), so a
// *Store satisfies such an interface as it is.
func (st *Store) Unread(scope, hunkID string) bool {
	if st == nil {
		return true
	}

	s, err := st.Set(scope)
	if err != nil {
		return true
	}

	return !s.Has(ID(hunkID))
}

// Save writes a scope's marks, keeping only the hunks live still carries; see the package Save.
// A scope never asked for has nothing to write.
func (st *Store) Save(scope string, live []Ref) error {
	st.mu.Lock()
	s, ok := st.sets[scope]
	st.mu.Unlock()

	if !ok {
		return nil
	}

	path, err := st.Path(scope)
	if err != nil {
		return err
	}

	return Save(path, s, live)
}

// Mark sets hunks of a scope read or unread and saves the scope at once, which is what a key that
// marks a hunk, a file, or a whole change wants: the mark is on disk before the next keystroke,
// where another process will find it. Pass the scope's whole diff as live, never a filtered view
// of it, since the save drops marks for hunks live does not carry.
func (st *Store) Mark(scope string, read bool, live []Ref, ids ...ID) error {
	s, err := st.Set(scope)
	if err != nil {
		return err
	}

	s.Mark(read, ids...)

	return st.Save(scope, live)
}

// Reload drops a scope's Set so the next ask reads the file again, which is how a long-running
// screen picks up marks another process saved without saving anything itself. Marks made through
// the dropped Set and not yet saved are lost.
func (st *Store) Reload(scope string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	delete(st.sets, scope)
}

const hexDigits = "0123456789ABCDEF"

func escape(scope string) string {
	var b strings.Builder

	for i := range len(scope) {
		c := scope[i]

		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '.', c == '_', c == '-':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hexDigits[c>>4])
			b.WriteByte(hexDigits[c&0xF])
		}
	}

	return b.String()
}
