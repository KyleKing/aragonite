package vimtext_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/tui/vimtext"
)

// stub writes an executable script from a shell of its own, so no child of
// the test binary holds it open for writing (golang/go#22315).
func stub(t *testing.T, script string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "editor")
	//nolint:gosec // G204: sh with a fixed script, the path a temporary directory of this test's
	cmd := exec.CommandContext(context.Background(), "sh", "-c", `cat > "$1" && chmod 700 "$1"`, "sh", path)
	cmd.Stdin = strings.NewReader(script)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("writing the stub: %v %s", err, out)
	}

	return path
}

// TestEdit_RoundTrip runs the editor Edit names on a temporary file holding the
// text and answers what it left there, and keeps the draft when the editor
// fails. $VISUAL wins over $EDITOR.
func TestEdit_RoundTrip(t *testing.T) {
	for _, c := range []struct {
		name, script, want string
		fails              bool
	}{
		{"edits", "#!/bin/sh\nprintf ' and more' >> \"$1\"\n", "draft and more", false},
		{"fails", "#!/bin/sh\nprintf 'lost' > \"$1\"\nexit 1\n", "draft", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("VISUAL", stub(t, c.script))
			t.Setenv("EDITOR", "false")

			if got := vimtext.Editor(); !strings.HasSuffix(got, "editor") {
				t.Fatalf("Editor is %q", got)
			}

			cmd, done, err := vimtext.EditCmd("id", "draft", "vimtext-*.md")
			if err != nil {
				t.Fatal(err)
			}

			msg, ok := done(cmd.Run()).(vimtext.EditedMsg)
			if !ok || msg.ID != "id" || msg.Text != c.want || (msg.Err != nil) != c.fails {
				t.Fatalf("answered %+v", msg)
			}
		})
	}
}

// TestEdit_NoFile answers the draft and the reason when the file cannot be
// made, without running anything.
func TestEdit_NoFile(t *testing.T) {
	t.Parallel()

	msg, ok := vimtext.Edit("id", "draft", "bad/pattern-*")().(vimtext.EditedMsg)
	if !ok || msg.Err == nil || msg.Text != "draft" {
		t.Fatalf("answered %+v", msg)
	}
}
