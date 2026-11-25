package vimtext

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

// EditedMsg is the text back from [Edit]. When the editor could not run, exited
// with a failure (vim's :cq), or its file could not be read back, Err says why
// and Text is the text as it was handed over, so the draft is never lost.
type EditedMsg struct {
	Err  error
	ID   string
	Text string
}

// Editor is the command line $VISUAL names, then $EDITOR, then vi.
func Editor() string {
	for _, v := range []string{"VISUAL", "EDITOR"} {
		if e := os.Getenv(v); e != "" {
			return e
		}
	}

	return "vi"
}

// Edit hands text to the user's editor ([Editor]) in a temporary file named by
// pattern (as os.CreateTemp takes it, such as "jojo-*.md"), with the terminal
// handed over through tea.ExecProcess, and answers an EditedMsg carrying id.
// The editor's command line is split by sh, so EDITOR="code --wait" works.
func Edit(id, text, pattern string) tea.Cmd {
	cmd, done, err := editCmd(id, text, pattern)
	if err != nil {
		return func() tea.Msg { return EditedMsg{ID: id, Text: text, Err: err} }
	}

	return tea.ExecProcess(cmd, done)
}

// editCmd is the command Edit runs and what answers once it exits.
func editCmd(id, text, pattern string) (*exec.Cmd, func(error) tea.Msg, error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("creating a file to edit: %w", err)
	}

	path := file.Name()

	_, werr := file.WriteString(text)
	if err := errors.Join(werr, file.Close()); err != nil {
		return nil, nil, fmt.Errorf("writing a file to edit: %w", errors.Join(err, os.Remove(path)))
	}

	editor := Editor()

	// The editor is a command line the user wrote, so a shell splits it, and
	// the file is its argument.
	//nolint:gosec // G204: the user's own editor setting, run on a file this program wrote.
	cmd := exec.CommandContext(context.Background(), "sh", "-c", editor+` "$1"`, "sh", path)

	done := func(runErr error) tea.Msg {
		b, readErr := os.ReadFile(path) //nolint:gosec // G304: the file written above.
		//nolint:errcheck,gosec // a temporary file that outlives the edit is not worth losing the edit over
		os.Remove(path)

		if err := errors.Join(runErr, readErr); err != nil {
			return EditedMsg{ID: id, Text: text, Err: fmt.Errorf("editing in %s: %w", editor, err)}
		}

		return EditedMsg{ID: id, Text: string(b)}
	}

	return cmd, done, nil
}
