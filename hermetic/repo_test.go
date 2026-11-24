package hermetic_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/hermetic"
)

// childEnv names the variable that turns TestChild on in a test binary the parent starts.
const childEnv = "HERMETIC_TEST_CHILD"

// GitRepoEnv holds every variable git itself names as locating a repository, so a newer git that
// adds one fails here rather than in a hook.
func TestGitRepoEnv_CoversGitsOwnList(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	out, err := exec.CommandContext(t.Context(), "git", "rev-parse", "--local-env-vars").Output()
	if err != nil {
		t.Fatalf("git rev-parse --local-env-vars: %v", err)
	}

	for key := range strings.FieldsSeq(string(out)) {
		if !slices.Contains(hermetic.GitRepoEnv, key) {
			t.Errorf("git names %s as repository-local and GitRepoEnv leaves it out", key)
		}
	}
}

// Under Main no repository-locating variable survives into the tests.
func TestIsolate_UnsetsGitRepoEnv(t *testing.T) {
	t.Parallel()

	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if slices.Contains(hermetic.GitRepoEnv, key) || strings.HasPrefix(key, "GIT_CONFIG_KEY_") {
			t.Errorf("%s survives Isolate", kv)
		}
	}
}

// State reads a staged change, a rewritten config, a new branch, and a moved HEAD, and not the
// stat refresh `git status` writes into the index.
func TestRepoState_ReadsWhatATestCouldWrite(t *testing.T) {
	t.Parallel()

	requireGit(t)

	dir := scratchRepo(t)
	repo := hermetic.Repo{GitDir: filepath.Join(dir, ".git"), WorkTree: dir}
	before := repo.State(t.Context())

	git(t, dir, nil, "status")

	if parts := before.Changed(repo.State(t.Context())); len(parts) != 0 {
		t.Errorf("git status reads as a change to %q", parts)
	}

	write(t, filepath.Join(dir, "a.txt"), "reverted\n")
	git(t, dir, nil, "add", "a.txt")

	after := repo.State(t.Context())
	if parts := before.Changed(after); !slices.Equal(parts, []string{"index"}) {
		t.Errorf("staging a file reads as a change to %q", parts)
	}

	before = after

	steps := []struct {
		part string
		args []string
	}{
		{"config", []string{"config", "core.worktree", "/elsewhere"}},
		{"refs", []string{"branch", "other"}},
		{"HEAD", []string{"symbolic-ref", "HEAD", "refs/heads/other"}},
	}

	for _, step := range steps {
		git(t, dir, nil, step.args...)

		after := repo.State(t.Context())
		if parts := before.Changed(after); !slices.Contains(parts, step.part) {
			t.Errorf("git %v reads as a change to %q, want %s", step.args, parts, step.part)
		}

		before = after
	}
}

// A test binary run from a git hook inherits GIT_DIR, GIT_WORK_TREE, and GIT_INDEX_FILE naming
// the repository being pushed. Its fixtures commit in temp directories of their own and leave that
// repository as it was, where without Main the same fixture writes into it.
func TestRun_KeepsAHooksRepositoryUntouched(t *testing.T) {
	t.Parallel()

	requireGit(t)

	scratch := scratchRepo(t)
	repo := hermetic.Repo{GitDir: filepath.Join(scratch, ".git"), WorkTree: scratch}
	hook := hookEnv(scratch)
	before := repo.State(t.Context())

	if out, err := child(t, "commit", hook); err != nil {
		t.Fatalf("the child run failed: %v\n%s", err, out)
	}

	if parts := before.Changed(repo.State(t.Context())); len(parts) != 0 {
		t.Errorf("the child's fixtures changed the hook's repository: %q", parts)
	}

	// The control: the fixture's first two steps with the hook's environment and no Main. (Its
	// commit would fail, since the work tree it stages from is the hook's.)
	control := t.TempDir()
	git(t, control, hook, "init", "--quiet", ".")
	git(t, control, hook, "config", "user.name", "Fixture")

	if parts := before.Changed(repo.State(t.Context())); len(parts) == 0 {
		t.Errorf("without Main the fixture left the hook's repository alone, so this test proves nothing")
	}
}

// A test that writes into the hook's repository anyway fails the run and names the repository.
func TestRun_FailsARunThatChangesTheRepository(t *testing.T) {
	t.Parallel()

	requireGit(t)

	scratch := scratchRepo(t)

	out, err := child(t, "leak", append(hookEnv(scratch), "HERMETIC_TEST_SCRATCH="+scratch))
	if err == nil {
		t.Fatalf("the run passed after writing core.worktree into %s\n%s", scratch, out)
	}

	if want := "changed the real repository " + filepath.Join(scratch, ".git"); !strings.Contains(out, want) ||
		!strings.Contains(out, "config") {
		t.Errorf("the failure does not say %q naming config:\n%s", want, out)
	}
}

// TestChild is the test binary's body when a test above starts it: off unless childEnv is set.
func TestChild(t *testing.T) {
	t.Parallel()

	switch os.Getenv(childEnv) {
	case "commit":
		fixture(t, t.TempDir(), nil)
	case "leak":
		git(t, os.Getenv("HERMETIC_TEST_SCRATCH"), nil, "config", "core.worktree", "/elsewhere")
	default:
		t.Skip("run by TestRun_* as a child")
	}
}

// child runs this test binary's TestChild in mode with env added, from a directory in no repository.
func child(t *testing.T, mode string, env []string) (string, error) {
	t.Helper()

	//nolint:gosec // G204: the test binary itself, with a fixed test name
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestChild$", "-test.count=1", "-test.v")
	cmd.Dir = t.TempDir()
	// A home of the child's own: the parent's is shared with tests running beside this one, and
	// the child guards whatever home it inherits.
	home := t.TempDir()
	cmd.Env = append(os.Environ(), childEnv+"="+mode, "HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_STATE_HOME="+filepath.Join(home, "state"),
		"XDG_DATA_HOME="+filepath.Join(home, "share"), "XDG_CACHE_HOME="+filepath.Join(home, ".cache"),
		"GH_CONFIG_DIR="+filepath.Join(home, ".config", "gh"))
	cmd.Env = append(cmd.Env, env...)

	var out bytes.Buffer

	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()

	return out.String(), err
}

// hookEnv is what git sets for a hook run in the repository at dir.
func hookEnv(dir string) []string {
	gitDir := filepath.Join(dir, ".git")

	return []string{
		"GIT_DIR=" + gitDir, "GIT_WORK_TREE=" + dir, "GIT_INDEX_FILE=" + filepath.Join(gitDir, "index"),
		"GIT_PREFIX=",
	}
}

// fixture is what a test's repository fixture does: init, configure, write, stage, commit.
func fixture(t *testing.T, dir string, env []string) {
	t.Helper()

	git(t, dir, env, "init", "--quiet", ".")
	git(t, dir, env, "config", "user.name", "Fixture")
	write(t, filepath.Join(dir, "a.txt"), "fixture\n")
	git(t, dir, env, "add", "a.txt")
	git(t, dir, env, "commit", "--quiet", "-m", "fixture")
}

func scratchRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	git(t, dir, nil, "init", "--quiet", ".")
	write(t, filepath.Join(dir, "a.txt"), "scratch\n")
	git(t, dir, nil, "add", "a.txt")
	git(t, dir, nil, "commit", "--quiet", "-m", "scratch")

	return dir
}

func git(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()

	//nolint:gosec // G204: fixture helper runs the literal git binary with test arguments.
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func requireGit(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}
