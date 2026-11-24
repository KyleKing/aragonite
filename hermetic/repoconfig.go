package hermetic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ErrOutsideHome marks a repository config jj would keep outside the run's root.
var ErrOutsideHome = errors.New("jj keeps this repository's config outside the test's home")

// RepoConfig sets the config of the jj repository at dir to toml, replacing what it held, by
// writing the file jj reads it from rather than going through `jj config set --repo`. Since jj 0.47
// that file is under $XDG_CONFIG_HOME/jj/repos, so it refuses a path outside the run's root: a
// test that did not go through Main would otherwise write into the real home. A test may point
// XDG_CONFIG_HOME at a directory of its own under the root.
func RepoConfig(tb testing.TB, dir, toml string) {
	tb.Helper()

	path, err := repoConfigPath(tb.Context(), dir)
	if err != nil {
		tb.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		tb.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(toml+"\n"), fileMode); err != nil {
		tb.Fatalf("writing %s: %v", path, err)
	}
}

func repoConfigPath(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "jj", "config", "path", "--repo")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("jj config path --repo: %w", err)
	}

	path := strings.TrimSpace(string(out))
	if !Within(path) {
		return "", fmt.Errorf("%w: %s; call hermetic.Main from TestMain", ErrOutsideHome, path)
	}

	return path, nil
}

// Within reports whether path is under the run's root, false when the process is not under Run.
func Within(path string) bool {
	root := os.Getenv(RootEnv)
	if root == "" || !strings.HasPrefix(filepath.Base(root), rootPrefix) {
		return false
	}

	for _, r := range []string{root, resolve(root)} {
		for _, p := range []string{path, resolve(path)} {
			if rel, err := filepath.Rel(r, p); err == nil && rel != ".." &&
				!strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}

	return false
}

// resolve is path with symlinks resolved as far as it exists, else path cleaned.
func resolve(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}

	return filepath.Clean(path)
}
