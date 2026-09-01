package ghcassette_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/ghcassette"
)

func TestMain(m *testing.M) {
	code := m.Run()

	ghcassette.RemoveStub()
	os.Exit(code)
}

// jobsPath is the gh api path for a run's jobs, the call every log read starts with.
const jobsPath = "repos/o/r/actions/runs/42/jobs"

// fakeGH echoes its arguments and exits with the code the caller asked for, so
// a record-then-replay round trip can be checked without touching GitHub.
func fakeGH(t *testing.T, exit int) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fake-gh")
	script := "#!/bin/sh\necho \"args: $*\"\necho 'a warning' >&2\nexit " + strconv.Itoa(exit) + "\n"

	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec // an executable is the point
		t.Fatalf("writing the fake gh: %v", err)
	}

	return path
}

func runGH(t *testing.T, s *ghcassette.Session, ghPath string, args ...string) (string, string, int) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), s.GH(), args...) // #nosec G204 -- the stub this package built
	cmd.Env = append(s.Env(t), "GH_CASSETTE_REAL="+ghPath)

	var out, errOut strings.Builder

	cmd.Stdout = &out
	cmd.Stderr = &errOut

	code := 0

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running the stub: %v (stderr: %s)", err, errOut.String())
		}

		code = exitErr.ExitCode()
	}

	return out.String(), errOut.String(), code
}

// TestRoundTrip records two gh calls against a fake binary, then replays them
// with a real path that does not exist, which is the guarantee the harness
// rests on: replay reaches nothing.
func TestRoundTrip(t *testing.T) {
	cassette := filepath.Join(t.TempDir(), "round-trip.golden")

	t.Setenv(ghcassette.RecordEnv, "1")

	rec := ghcassette.Start(t, cassette)
	if !rec.Recording() {
		t.Fatal("expected a recording session")
	}

	ghPath := fakeGH(t, 3)

	out, errOut, code := runGH(t, rec, ghPath, "run", "view", "42", "--log")
	if out != "args: run view 42 --log\n" || errOut != "a warning\n" || code != 3 {
		t.Fatalf("record: out %q err %q exit %d", out, errOut, code)
	}

	const jobsOut = "args: api repos/o/r/actions/runs/42/jobs\n"

	if out, _, _ := runGH(t, rec, ghPath, "api", jobsPath); out != jobsOut {
		t.Fatalf("record: got %q", out)
	}

	t.Setenv(ghcassette.RecordEnv, "0")

	play := ghcassette.Start(t, cassette)

	out, errOut, code = runGH(t, play, "/nonexistent/gh", "run", "view", "42", "--log")
	if out != "args: run view 42 --log\n" || errOut != "a warning\n" || code != 3 {
		t.Fatalf("replay: out %q err %q exit %d", out, errOut, code)
	}

	if out, _, _ := runGH(t, play, "/nonexistent/gh", "api", jobsPath); out != jobsOut {
		t.Fatalf("replay: got %q", out)
	}

	play.RequireAllPlayed(t)
}

// A program that makes several calls at once makes them in whatever order the
// scheduler picks, so replay answers each by the first interaction matching it
// that nothing has taken yet, rather than by a cursor that only moves forward.
func TestReplayAnswersCallsMadeOutOfOrder(t *testing.T) {
	cassette := filepath.Join(t.TempDir(), "out-of-order.golden")

	t.Setenv(ghcassette.RecordEnv, "1")

	rec := ghcassette.Start(t, cassette)
	ghPath := fakeGH(t, 0)

	runGH(t, rec, ghPath, "search", "prs", "--first")
	runGH(t, rec, ghPath, "search", "prs", "--second")
	runGH(t, rec, ghPath, "search", "prs", "--third")

	t.Setenv(ghcassette.RecordEnv, "0")

	play := ghcassette.Start(t, cassette)

	for _, which := range []string{"--third", "--first", "--second"} {
		want := "args: search prs " + which + "\n"
		if out, _, _ := runGH(t, play, "/nonexistent/gh", "search", "prs", which); out != want {
			t.Fatalf("replaying %s out of order gave %q", which, out)
		}
	}

	play.RequireAllPlayed(t)
}

// TestReplayRejectsAnUnrecordedCall is what stops a cassette drifting behind
// the code: a call nobody recorded fails loudly rather than returning nothing,
// which would read as a run with no logs.
func TestReplayRejectsAnUnrecordedCall(t *testing.T) {
	t.Parallel()

	cassette := filepath.Join(t.TempDir(), "empty.golden")
	if err := ghcassette.Save(cassette, &ghcassette.Cassette{}); err != nil {
		t.Fatal(err)
	}

	s := ghcassette.Replay(t, cassette)

	_, errOut, code := runGH(t, s, "/nonexistent/gh", "run", "view", "42", "--log")
	if code == 0 {
		t.Fatal("an unrecorded call succeeded")
	}

	if !strings.Contains(errOut, "no recorded gh interaction matches") {
		t.Errorf("stderr does not name the missing interaction: %q", errOut)
	}
}

// TestRequireAllPlayedCatchesADroppedCall guards the other direction: code that
// stops making a call it used to make leaves an interaction behind.
func TestRequireAllPlayedCatchesADroppedCall(t *testing.T) {
	t.Parallel()

	cassette := filepath.Join(t.TempDir(), "two-calls.golden")

	err := ghcassette.Save(cassette, &ghcassette.Cassette{Interactions: []ghcassette.Interaction{
		{Args: []string{"run", "view", "1"}, Stdout: "one\n"},
		{Args: []string{"run", "view", "2"}, Stdout: "two\n"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	s := ghcassette.Replay(t, cassette)

	if out, _, _ := runGH(t, s, "/nonexistent/gh", "run", "view", "1"); out != "one\n" {
		t.Fatalf("got %q", out)
	}

	if !failsRequireAllPlayed(t, s) {
		t.Error("a cassette with an unplayed interaction passed RequireAllPlayed")
	}
}

// failsRequireAllPlayed reports whether RequireAllPlayed rejected the session,
// catching the Fatalf it raises so the assertion can be made on it.
func failsRequireAllPlayed(t *testing.T, s *ghcassette.Session) bool {
	t.Helper()

	failed := false

	func() {
		defer func() {
			failed = recover() != nil
		}()

		s.RequireAllPlayed(&recordingTB{TB: t})
	}()

	return failed
}

// recordingTB turns the failure RequireAllPlayed reports into a panic, so the
// test asserting on it does not fail itself.
type recordingTB struct {
	testing.TB
}

func (*recordingTB) Fatalf(string, ...any) {
	panic(errStopTB)
}

var errStopTB = errors.New("stop")

// echoingGH writes whatever it read on stdin back to stdout, so a round trip
// can tell a recorded body from an empty one.
func echoingGH(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "echo-gh")
	//nolint:gosec // an executable is the point
	if err := os.WriteFile(path, []byte("#!/bin/sh\ncat\n"), 0o700); err != nil {
		t.Fatalf("writing the fake gh: %v", err)
	}

	return path
}

func runGHWithStdin(t *testing.T, s *ghcassette.Session, ghPath, stdin string, args ...string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), s.GH(), args...) // #nosec G204 -- the stub this package built
	cmd.Env = append(s.Env(t), "GH_CASSETTE_REAL="+ghPath)
	cmd.Stdin = strings.NewReader(stdin)

	var out strings.Builder

	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		t.Fatalf("running the stub: %v", err)
	}

	return out.String()
}

// TestRecordsTheBodyGHWasGiven covers a call that posts rather than reads.
// The body is recorded for the reader rather than matched on, so a cassette
// says what was sent without the args alone having to carry it.
func TestRecordsTheBodyGHWasGiven(t *testing.T) {
	cassette := filepath.Join(t.TempDir(), "with-stdin.golden")
	body := `{"body":"looks good"}`

	t.Setenv(ghcassette.RecordEnv, "1")

	recording := ghcassette.Start(t, cassette)

	got := runGHWithStdin(t, recording, echoingGH(t), body,
		"api", "repos/o/r/issues/1/comments", "--input", "-")
	if got != body {
		t.Fatalf("recording returned %q, want the body it was given", got)
	}

	loaded, err := ghcassette.Load(cassette)
	if err != nil {
		t.Fatalf("loading the cassette: %v", err)
	}

	if len(loaded.Interactions) != 1 {
		t.Fatalf("recorded %d interactions, want 1", len(loaded.Interactions))
	}

	if loaded.Interactions[0].Stdin != body {
		t.Errorf("recorded stdin is %q, want %q", loaded.Interactions[0].Stdin, body)
	}

	t.Setenv(ghcassette.RecordEnv, "0")

	replaying := ghcassette.Replay(t, cassette)

	if got := runGHWithStdin(t, replaying, "/nonexistent/gh", body,
		"api", "repos/o/r/issues/1/comments", "--input", "-"); got != body {
		t.Errorf("replay returned %q, want the recorded response", got)
	}

	replaying.RequireAllPlayed(t)
}

// TestStdinIsLeftAloneWithoutInputDash guards the hang: gh inherits the
// caller's stdin for every other call, and reading a terminal would block
// forever.
func TestStdinIsLeftAloneWithoutInputDash(t *testing.T) {
	cassette := filepath.Join(t.TempDir(), "no-stdin.golden")

	t.Setenv(ghcassette.RecordEnv, "1")

	recording := ghcassette.Start(t, cassette)
	runGH(t, recording, fakeGH(t, 0), "api", jobsPath)

	loaded, err := ghcassette.Load(cassette)
	if err != nil {
		t.Fatalf("loading the cassette: %v", err)
	}

	if loaded.Interactions[0].Stdin != "" {
		t.Errorf("recorded stdin %q for a call that reads none", loaded.Interactions[0].Stdin)
	}
}
