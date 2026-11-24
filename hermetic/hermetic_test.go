package hermetic_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/hermetic"
)

func TestMain(m *testing.M) { hermetic.Main(m, hermetic.App("aragonite")) }

// Every process a test starts finds the isolated home, and jj's per-repository records land in it.
func TestIsolate_PointsJJAtTheTestHome(t *testing.T) {
	t.Parallel()

	root := os.Getenv(hermetic.RootEnv)
	home := os.Getenv("HOME")

	if root == "" || !strings.HasPrefix(home, root) {
		t.Fatalf("HOME is %s, not under the run's root %q", home, root)
	}

	for _, key := range []string{
		"XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME",
		"JJ_CONFIG", "GH_CONFIG_DIR", "GIT_CONFIG_GLOBAL",
	} {
		if !strings.HasPrefix(os.Getenv(key), home) {
			t.Errorf("%s is %s, outside %s", key, os.Getenv(key), home)
		}
	}

	if dir := t.TempDir(); !hermetic.Within(dir) {
		t.Errorf("t.TempDir is %s, outside the run's root", dir)
	}

	if hermetic.Within(filepath.Dir(root)) {
		t.Errorf("the root's parent reads as inside the root")
	}

	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj is not installed")
	}

	dir := t.TempDir()
	run(t, dir, "git", "init", ".")
	hermetic.RepoConfig(t, dir, "[ui]\ndiff-editor = \":builtin\"")

	if got := strings.TrimSpace(run(t, dir, "config", "get", "ui.diff-editor")); got != ":builtin" {
		t.Errorf("the repository config reads %q", got)
	}

	path := strings.TrimSpace(run(t, dir, "config", "path", "--repo"))
	if !strings.HasPrefix(path, home) {
		t.Errorf("jj keeps the repository config at %s, outside %s", path, home)
	}

	if got := strings.TrimSpace(run(t, dir, "config", "get", "user.name")); got != hermetic.User {
		t.Errorf("jj commits as %q", got)
	}
}

// A guarded path changing is a leak, except a jj repository record that names no repository of
// this run's, which another program opening its own repository writes.
func TestLeaks_OnlyTheRunsOwnRecordsCount(t *testing.T) {
	t.Parallel()

	jjDir, root := filepath.Join(t.TempDir(), "jj"), "/tmp/hermetic-123"
	write(t, filepath.Join(jjDir, "config.toml"), "[ui]\n")
	write(t, filepath.Join(jjDir, "repos", "old", "metadata.binpb"), "/home/me/project/.jj/repo")

	before := hermetic.Take(jjDir)

	write(t, filepath.Join(jjDir, "repos", "ours", "metadata.binpb"), root+"/tmp/TestX/001/.jj/repo")
	write(t, filepath.Join(jjDir, "repos", "theirs", "metadata.binpb"), "/home/me/other/.jj/repo")

	if leaks := hermetic.Leaks(before, hermetic.Take(jjDir), jjDir, root); !slices.Equal(leaks, []string{
		filepath.Join(jjDir, "repos", "ours"),
		filepath.Join(jjDir, "repos", "ours", "metadata.binpb"),
	}) {
		t.Errorf("leaks %q", leaks)
	}

	before = hermetic.Take(jjDir)
	write(t, filepath.Join(jjDir, "config.toml"), "[ui]\npager = \"less\"\n")

	if leaks := hermetic.Leaks(before, hermetic.Take(jjDir), jjDir, root); len(leaks) != 1 ||
		leaks[0] != filepath.Join(jjDir, "config.toml") {
		t.Errorf("a rewritten user config leaks %q", leaks)
	}

	missing := filepath.Join(t.TempDir(), "gh")
	before = hermetic.Take(missing)
	write(t, filepath.Join(missing, "hosts.yml"), "x")

	if leaks := hermetic.Leaks(before, hermetic.Take(missing), jjDir, root); len(leaks) != 2 {
		t.Errorf("creating a guarded directory leaks %q", leaks)
	}
}

// App guards each of a tool's XDG directories, and nothing of another tool's.
func TestPaths_GuardedNamesTheAppsDirectories(t *testing.T) {
	t.Parallel()

	p := hermetic.Paths{
		Home: "/h", Config: "/h/.config", State: "/h/.local/state", Data: "/h/.local/share",
		Cache: "/h/.cache", JJ: "/h/.config/jj", GH: "/h/.config/gh",
	}

	got := p.Guarded("tool")
	for _, want := range []string{
		"/h/.config/jj", "/h/.gitconfig", "/h/.config/git", "/h/.config/gh",
		"/h/.config/tool", "/h/.local/state/tool", "/h/.cache/tool", "/h/.local/share/tool",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("Guarded leaves out %s: %q", want, got)
		}
	}

	if len(p.Guarded()) != 4 {
		t.Errorf("with no app Guarded is %q", p.Guarded())
	}
}

// A package with tests and no guard is named; a guarded one, testdata, and a nested module are not.
func TestUnguarded(t *testing.T) {
	t.Parallel()

	module := t.TempDir()
	write(t, filepath.Join(module, "go.mod"), "module x\n")
	write(t, filepath.Join(module, "a", "a_test.go"), "package a\n")
	write(t, filepath.Join(module, "b", "b_test.go"), "package b\n")
	write(t, filepath.Join(module, "b", "main_test.go"), "func TestMain(m *testing.M) { hermetic.Main(m) }\n")
	write(t, filepath.Join(module, "c", "c_test.go"), "code := hermetic.Run(m)\n")
	write(t, filepath.Join(module, "d", "testdata", "x_test.go"), "package x\n")
	write(t, filepath.Join(module, "e", "go.mod"), "module e\n")
	write(t, filepath.Join(module, "e", "e_test.go"), "package e\n")
	write(t, filepath.Join(module, "f", "f.go"), "package f\n")

	got, err := hermetic.Unguarded(module)
	if err != nil || !slices.Equal(got, []string{"a"}) {
		t.Errorf("Unguarded = %q, %v; want [a]", got, err)
	}
}

// Every package in this module installs the guard.
func TestEveryTestPackageIsHermetic(t *testing.T) {
	t.Parallel()

	hermetic.RequireEveryPackage(t, "..")
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()

	//nolint:gosec // G204: fixture helper runs the literal jj binary with test arguments.
	cmd := exec.CommandContext(t.Context(), "jj", args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jj %v: %v\n%s", args, err, out)
	}

	return string(out)
}

func write(t *testing.T, path, text string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
