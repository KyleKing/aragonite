package hermetic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// GitRepoEnv is every variable that tells git which repository, work tree, index, or object store
// to use: what `git rev-parse --local-env-vars` prints, plus GIT_NAMESPACE, which moves every ref.
// Hooks run with GIT_DIR set (and in a linked work tree, more of these), so a `go test` under a
// pre-commit or pre-push hook inherits them, and every fixture `git init` or `git commit` in a temp
// directory would then write into the repository being pushed. Isolate unsets
// each of them, and the GIT_CONFIG_KEY_<n> and GIT_CONFIG_VALUE_<n> pairs GIT_CONFIG_COUNT counts.
var GitRepoEnv = []string{ //nolint:gochecknoglobals // a fixed list, read by tests and Isolate
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_CONFIG",
	"GIT_CONFIG_PARAMETERS",
	"GIT_CONFIG_COUNT",
	"GIT_OBJECT_DIRECTORY",
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_IMPLICIT_WORK_TREE",
	"GIT_GRAFT_FILE",
	"GIT_INDEX_FILE",
	"GIT_NO_REPLACE_OBJECTS",
	"GIT_REPLACE_REF_BASE",
	"GIT_PREFIX",
	"GIT_SHALLOW_FILE",
	"GIT_COMMON_DIR",
	"GIT_NAMESPACE",
}

const gitTimeout = 30 * time.Second

// repoEnv reports whether key is one of GitRepoEnv or a GIT_CONFIG_KEY_/GIT_CONFIG_VALUE_ pair.
func repoEnv(key string) bool {
	return slices.Contains(GitRepoEnv, key) ||
		strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_")
}

// unsetRepoEnv removes every repository-locating git variable from the process.
func unsetRepoEnv() error {
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if !repoEnv(key) {
			continue
		}

		if err := os.Unsetenv(key); err != nil {
			return fmt.Errorf("unsetting %s: %w", key, err)
		}
	}

	return nil
}

// cleanEnv is env without the repository-locating git variables, and with git told to take no
// optional lock, so reading a repository never refreshes its index.
func cleanEnv(env []string) []string {
	out := make([]string, 0, len(env)+1)

	for _, kv := range env {
		if key, _, _ := strings.Cut(kv, "="); !repoEnv(key) {
			out = append(out, kv)
		}
	}

	return append(out, "GIT_OPTIONAL_LOCKS=0")
}

// Repo is a git repository a run must leave as it found it: its git directory and the work tree
// its index describes.
type Repo struct{ GitDir, WorkTree string }

// RealRepos is every repository a test could write into by accident: the one git would open from
// the working directory with the inherited environment (the repository a hook runs for, when
// GIT_DIR is set), and the one holding the working directory. It reads them before Isolate, and
// answers none where git is missing or the directory is in no repository.
func RealRepos(ctx context.Context) []Repo {
	var out []Repo

	for _, env := range [][]string{os.Environ(), cleanEnv(os.Environ())} {
		text, err := gitOutput(ctx, env, "rev-parse", "--absolute-git-dir", "--show-toplevel")
		if err != nil {
			continue
		}

		lines := strings.Split(strings.TrimSpace(text), "\n")
		if len(lines) != 2 { //nolint:mnd // one line for each flag
			continue
		}

		repo := Repo{GitDir: lines[0], WorkTree: lines[1]}
		if !slices.Contains(out, repo) {
			out = append(out, repo)
		}
	}

	return out
}

// RepoState is what a run must not change in a repository, by part: its config, HEAD, local
// branches, tags, and stash, and its index as the entries it stages (path, mode, and blob, never
// the stat data `git status` refreshes). Remote-tracking refs are left out, since a fetch in
// another terminal moves them.
type RepoState map[string]string

// State reads r's RepoState. A part git cannot read is recorded as its error, so a repository
// that later reads differently is still a change.
func (r Repo) State(ctx context.Context) RepoState {
	env := cleanEnv(os.Environ())
	s := RepoState{}

	for _, name := range []string{"config", "HEAD"} {
		data, err := os.ReadFile(filepath.Join(r.GitDir, name)) //nolint:gosec // the repository's own file
		s[name] = digest(data, err)
	}

	if common, err := gitOutput(ctx, env, "--git-dir="+r.GitDir, "rev-parse", "--git-common-dir"); err == nil {
		common = strings.TrimSpace(common)
		if !filepath.IsAbs(common) {
			common = filepath.Join(r.GitDir, common)
		}

		if filepath.Clean(common) != filepath.Clean(r.GitDir) {
			data, err := os.ReadFile(filepath.Join(common, "config")) //nolint:gosec // as above
			s["common config"] = digest(data, err)
		}
	}

	refs, err := gitOutput(ctx, env, "--git-dir="+r.GitDir, "for-each-ref",
		"--format=%(refname) %(objectname)", "refs/heads", "refs/tags", "refs/stash")
	s["refs"] = digest([]byte(refs), err)

	index, err := gitOutput(ctx, env, "--git-dir="+r.GitDir, "--work-tree="+r.WorkTree, "ls-files", "--stage")
	s["index"] = digest([]byte(index), err)

	return s
}

// Changed lists the parts of the repository that differ between s and after, sorted.
func (s RepoState) Changed(after RepoState) []string {
	var out []string

	for part, was := range s {
		if after[part] != was {
			out = append(out, part)
		}
	}

	for part := range after {
		if _, ok := s[part]; !ok {
			out = append(out, part)
		}
	}

	slices.Sort(out)

	return out
}

func digest(data []byte, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

func gitOutput(ctx context.Context, env []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // G204: git, with arguments this file builds
	cmd.Env = env
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return string(out), nil
}
