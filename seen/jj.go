package seen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// JJApp is the state directory every tool keeps a jj repository's read marks under, named for
// jojo, where marks are made. jojo and jj-diff scope it by change id.
const JJApp = "jojo"

// OpenJJ is the read-mark store for the jj repository dir is in, whichever workspace or
// subdirectory dir is: $XDG_STATE_HOME/jojo/seen/<RepoKey(JJRepo(dir))>. Every tool reading a jj
// repository's marks opens it this way, which is the one thing that lets jojo, jj-diff, and
// second-look agree on a directory; its scopes are jj change ids. A directory in no jj workspace
// is ErrNoJJ.
func OpenJJ(dir string) (*Store, error) {
	repo, err := JJRepo(dir)
	if err != nil {
		return nil, err
	}

	state, err := StateDir(JJApp, "seen", RepoKey(repo))
	if err != nil {
		return nil, err
	}

	return Open(state), nil
}

// ErrNoJJ is returned by JJRepo for a directory that is in no jj workspace.
var ErrNoJJ = errors.New("not in a jj workspace")

// errEmptyPointer marks a secondary workspace whose `.jj/repo` names nothing.
var errEmptyPointer = errors.New("names no repository")

// JJRepo is the repository a jj workspace belongs to, for RepoKey: the workspace is the nearest
// directory at or above dir holding a `.jj` directory, and its repository is `<workspace>/.jj/repo`
// where that is a directory (the main workspace) or the directory that file names, relative to
// `.jj`, where it is a file (a secondary workspace). Every workspace of a repository, and every
// directory inside one, answers the same cleaned absolute path with symlinks resolved, which is
// what lets two tools (jojo and jj-diff) agree on one directory of marks without asking jj.
func JJRepo(dir string) (string, error) {
	root, err := jjWorkspace(dir)
	if err != nil {
		return "", err
	}

	dotJJ := filepath.Join(root, ".jj")
	repo := filepath.Join(dotJJ, "repo")

	info, err := os.Stat(repo)
	if err != nil {
		return "", fmt.Errorf("finding the repository of %s: %w", root, err)
	}

	if info.IsDir() {
		return absClean(repo)
	}

	data, err := os.ReadFile(repo) //nolint:gosec // G304: the workspace's own pointer file.
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", repo, err)
	}

	target := strings.TrimSpace(string(data))
	if target == "" {
		return "", fmt.Errorf("%s: %w", repo, errEmptyPointer)
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(dotJJ, target)
	}

	return absClean(target)
}

// jjWorkspace is the nearest directory at or above dir that holds a `.jj` directory.
func jjWorkspace(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", dir, err)
	}

	for at := abs; ; {
		if info, err := os.Stat(filepath.Join(at, ".jj")); err == nil && info.IsDir() {
			return at, nil
		}

		up := filepath.Dir(at)
		if up == at {
			return "", fmt.Errorf("%s: %w", dir, ErrNoJJ)
		}

		at = up
	}
}

func absClean(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}

	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}

	return filepath.Clean(abs), nil
}
