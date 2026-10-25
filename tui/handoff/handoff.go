// Package handoff gives the terminal to another program and takes it back. The
// protocol is argv in, exit back: the caller's frame suspends, the child runs
// with the whole terminal in a directory, and on exit the caller gets a
// [DoneMsg] to redraw and refresh whatever the child may have changed.
//
// The command is a template for sh -c. Every substituted value is shell-quoted,
// because a git branch name can hold $, ;, and backticks.
package handoff

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

var (
	// ErrUnknownPlaceholder is a template naming a key the caller did not pass.
	ErrUnknownPlaceholder = errors.New("unknown placeholder")
	// ErrEmptyValue is a template using a key whose value is empty, such as {pr}
	// on a row with no pull request.
	ErrEmptyValue = errors.New("empty value for placeholder")
)

var placeholder = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// DoneMsg reports a finished hand-off. Err is the child's exit error, or nil.
type DoneMsg struct {
	Values   map[string]string
	Err      error
	Name     string
	Dir      string
	Duration time.Duration
}

// Needs lists the distinct placeholders a template uses, sorted.
func Needs(template string) []string {
	matches := placeholder.FindAllStringSubmatch(template, -1)
	names := make([]string, 0, len(matches))

	for _, m := range matches {
		names = append(names, m[1])
	}

	slices.Sort(names)

	return slices.Compact(names)
}

// Expand substitutes each {key} with its shell-quoted value. A key missing from
// values or holding an empty value is an error naming it.
func Expand(template string, values map[string]string) (string, error) {
	var failure error

	out := placeholder.ReplaceAllStringFunc(template, func(token string) string {
		name := token[1 : len(token)-1]

		value, ok := values[name]
		switch {
		case failure != nil:
			return ""
		case !ok:
			failure = fmt.Errorf("%w {%s}", ErrUnknownPlaceholder, name)
		case value == "":
			failure = fmt.Errorf("%w {%s}", ErrEmptyValue, name)
		}

		return quote(value)
	})
	if failure != nil {
		return "", failure
	}

	return out, nil
}

// Run expands template and returns the command that suspends the program, runs
// it under sh -c in dir with env ("KEY=value" entries) added to the
// environment, and answers with a [DoneMsg] named name. A template that fails
// to expand returns the error and no command, so nothing runs.
func Run(name, template string, values map[string]string, dir string, env []string) (tea.Cmd, error) {
	script, err := Expand(template, values)
	if err != nil {
		return nil, fmt.Errorf("hand-off %q: %w", name, err)
	}

	// #nosec G204 -- the template is the operator's own configuration and every
	// substituted value is quoted by Expand.
	cmd := exec.CommandContext(context.Background(), "sh", "-c", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	start := time.Now()

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return DoneMsg{
			Name:     name,
			Dir:      dir,
			Values:   values,
			Err:      err,
			Duration: time.Since(start),
		}
	}), nil
}

func quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
