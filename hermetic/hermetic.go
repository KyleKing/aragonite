// Package hermetic keeps a test binary off the account that runs it. Every process a test starts
// (jj, git, gh, the tool itself) and every path the tool reads through the environment lands in a
// home the binary made for itself, and the run fails when the real one changed anyway.
//
// jj 0.47 keeps a repository's config, and a record of every repository it opened, under
// $XDG_CONFIG_HOME/jj/repos rather than in the repository, so a fixture's `jj config set --repo`,
// or a .jj/repo/config.toml jj migrates on its next call, writes into whatever home the test
// inherited. Isolation has to come from the environment, which is what Run sets before any test
// starts. A package's TestMain is one line:
//
//	func TestMain(m *testing.M) { hermetic.Main(m, hermetic.App("jojo")) }
//
// and one test per module calls RequireEveryPackage, so a package added later without it fails.
package hermetic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// User is the name and email every jj and git call in a test commits as.
const (
	User  = "Test User"
	Email = "test@example.com"
)

// RootEnv names the variable Run sets to the run's root, the directory holding the isolated home
// and the temp directory. RepoConfig refuses a path outside it, and a test may read it to tell
// whether it runs under Main.
const RootEnv = "HERMETIC_ROOT"

const (
	dirMode  = 0o750
	fileMode = 0o600
	// Every run's root under the real temp directory is named starting with rootPrefix.
	rootPrefix = "hermetic-"
)

// Runner is what Run drives: *testing.M.
type Runner interface{ Run() int }

// Option adds to what Run guards.
type Option func(*settings)

type settings struct {
	apps  []string
	extra []string
}

// App guards a tool's own directories of the real account: its config, state, cache, and data
// directories, each <xdg dir>/<name>. Pass the name the tool files its state under.
func App(names ...string) Option {
	return func(s *settings) { s.apps = append(s.apps, names...) }
}

// Guard adds absolute paths of the real account, files or directories walked whole, that a run
// must leave as it found them.
func Guard(paths ...string) Option {
	return func(s *settings) { s.extra = append(s.extra, paths...) }
}

// Main is a package's TestMain: it runs the tests in a home of their own and exits with their
// code, or with a failure when one of them reached the real home.
func Main(m Runner, opts ...Option) { os.Exit(Run(m, opts...)) }

// Run isolates the process, runs m, and answers its exit code, which it turns into a failure when
// a guarded path of the real home changed and the change is this run's. A TestMain with cleanup of
// its own calls Run and exits after the cleanup.
func Run(m Runner, opts ...Option) int {
	var s settings
	for _, opt := range opts {
		opt(&s)
	}

	owner, err := RealPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "hermetic:", err)

		return 1
	}

	root, err := os.MkdirTemp("", rootPrefix)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hermetic:", err)

		return 1
	}

	defer os.RemoveAll(root) //nolint:errcheck // a temp dir left behind is harmless

	guarded := append(owner.Guarded(s.apps...), s.extra...)
	before := Take(guarded...)

	if err := Isolate(root, owner); err != nil {
		fmt.Fprintln(os.Stderr, "hermetic:", err)

		return 1
	}

	code := m.Run()

	if leaks := Leaks(before, Take(guarded...), owner.JJ, root); len(leaks) > 0 {
		fmt.Fprintf(os.Stderr, "hermetic: the tests changed the real home, which they must never touch:\n  %s\n",
			strings.Join(leaks, "\n  "))

		return 1
	}

	return code
}

// Paths are the real account's directories a test could reach through the environment.
type Paths struct {
	Home   string
	Config string // $XDG_CONFIG_HOME, or ~/.config
	State  string // $XDG_STATE_HOME, or ~/.local/state
	Data   string // $XDG_DATA_HOME, or ~/.local/share
	Cache  string // os.UserCacheDir
	JJ     string // jj's user config directory, which holds repos/
	GH     string // gh's config directory
}

// RealPaths reads the real account's directories from the environment as it is before Isolate.
func RealPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("finding the home directory: %w", err)
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		return Paths{}, fmt.Errorf("finding the cache directory: %w", err)
	}

	p := Paths{
		Home: home, Cache: cache,
		Config: envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config")),
		State:  envOr("XDG_STATE_HOME", filepath.Join(home, ".local", "state")),
		Data:   envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share")),
	}
	p.JJ = filepath.Join(p.Config, "jj")
	p.GH = envOr("GH_CONFIG_DIR", filepath.Join(p.Config, "gh"))

	return p, nil
}

// Guarded is every real path a test run must leave as it found it: jj's config and its
// per-repository records, git's global config, gh's config, and each app's config, state, cache,
// and data directories.
func (p Paths) Guarded(apps ...string) []string {
	out := make([]string, 0, 4+4*len(apps))
	out = append(out,
		p.JJ,
		filepath.Join(p.Home, ".gitconfig"), filepath.Join(p.Config, "git"),
		p.GH,
	)

	for _, app := range apps {
		out = append(out,
			filepath.Join(p.Config, app), filepath.Join(p.State, app),
			filepath.Join(p.Cache, app), filepath.Join(p.Data, app))
	}

	return out
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

// Isolate points the process at a home under root: HOME and every XDG directory, jj's and git's
// user config, gh's config, and the temp directory, so every t.TempDir and every repository a test
// makes is under root too. It names the committer for jj and git and sets RootEnv to root. Go's own
// caches stay where owner's are, since a cold build cache would rebuild the standard library for
// every test that builds a binary.
func Isolate(root string, owner Paths) error {
	home := filepath.Join(root, "home")
	tmp := filepath.Join(root, "tmp")
	config := filepath.Join(home, ".config")

	for _, dir := range []string{tmp, filepath.Join(config, "jj")} {
		if err := os.MkdirAll(dir, dirMode); err != nil {
			return fmt.Errorf("making %s: %w", dir, err)
		}
	}

	env := []struct{ key, value string }{
		{"GOCACHE", envOr("GOCACHE", filepath.Join(owner.Cache, "go-build"))},
		{"GOPATH", envOr("GOPATH", filepath.Join(owner.Home, "go"))},
		{"GOENV", envOr("GOENV", filepath.Join(owner.Config, "go", "env"))},
		{RootEnv, root},
		{"HOME", home},
		{"XDG_CONFIG_HOME", config},
		{"XDG_DATA_HOME", filepath.Join(home, ".local", "share")},
		{"XDG_STATE_HOME", filepath.Join(home, ".local", "state")},
		{"XDG_CACHE_HOME", filepath.Join(home, ".cache")},
		{"TMPDIR", tmp},
		{"JJ_CONFIG", filepath.Join(config, "jj", "config.toml")},
		{"JJ_USER", User},
		{"JJ_EMAIL", Email},
		{"GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig")},
		{"GIT_CONFIG_NOSYSTEM", "1"},
		{"GIT_AUTHOR_NAME", User},
		{"GIT_AUTHOR_EMAIL", Email},
		{"GIT_COMMITTER_NAME", User},
		{"GIT_COMMITTER_EMAIL", Email},
		{"GH_CONFIG_DIR", filepath.Join(config, "gh")},
	}

	for _, e := range env {
		if err := os.Setenv(e.key, e.value); err != nil {
			return fmt.Errorf("setting %s: %w", e.key, err)
		}
	}

	return nil
}
