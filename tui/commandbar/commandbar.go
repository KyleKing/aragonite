// Package commandbar is the registry behind a `:command` bar and the tab
// completion that reads it. It owns no input widget and no rendering, because
// the two screens that use it key and draw the bar differently; it is generic
// over the screen's model so it never names one.
package commandbar

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Command is one named `:command` the bar can run.
type Command[M any] struct {
	// Run applies the command. Args are the whitespace-separated words after
	// the command's own name.
	Run func(M, []string) (M, tea.Cmd)
	// Complete offers the candidates for the argument being typed, which is
	// the last element of args and may be empty. A nil Complete means the
	// command takes no arguments worth completing.
	Complete    func(M, []string) []Candidate
	Name        string
	Description string
}

// Candidate is one completion paired with the line explaining it, so the bar
// shows both without a second lookup.
type Candidate struct {
	Name        string
	Description string
}

// Registry holds the commands a bar knows, sorted by name.
type Registry[M any] struct {
	commands []Command[M]
}

// NewRegistry builds a Registry, sorting by name so every listing and
// completion comes back in one order.
func NewRegistry[M any](commands ...Command[M]) Registry[M] {
	sorted := make([]Command[M], len(commands))
	copy(sorted, commands)

	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	return Registry[M]{commands: sorted}
}

// Commands returns every registered command.
func (r Registry[M]) Commands() []Command[M] {
	return r.commands
}

// Lookup resolves a name exactly, or by a prefix that only one command
// answers to. An ambiguous prefix resolves to nothing rather than to the
// first match, so `:c` never silently runs whichever command sorts first.
func (r Registry[M]) Lookup(name string) (Command[M], bool) {
	var matches []Command[M]

	for _, command := range r.commands {
		if command.Name == name {
			return command, true
		}

		if strings.HasPrefix(command.Name, name) {
			matches = append(matches, command)
		}
	}

	if len(matches) == 1 {
		return matches[0], true
	}

	return Command[M]{}, false
}

// Candidates returns the commands whose names start with prefix.
func (r Registry[M]) Candidates(prefix string) []Candidate {
	candidates := make([]Candidate, 0, len(r.commands))

	for _, command := range r.commands {
		if strings.HasPrefix(command.Name, prefix) {
			candidates = append(candidates, Candidate{Name: command.Name, Description: command.Description})
		}
	}

	return candidates
}

// Listing renders every command as "name<tab>description", one per line, for
// a caller with no help modal to open.
func (r Registry[M]) Listing() string {
	var b strings.Builder

	for _, command := range r.commands {
		b.WriteString(command.Name)
		b.WriteString("\t")
		b.WriteString(command.Description)
		b.WriteString("\n")
	}

	return strings.TrimSuffix(b.String(), "\n")
}

// Completions returns the candidates for the word being typed in line, and
// reports whether anything can complete it. Completing the first word offers
// commands; completing a later one asks that command, and a command with no
// Complete reports false rather than offering command names again.
func (r Registry[M]) Completions(m M, line string) ([]Candidate, bool) {
	fields := strings.Fields(line)
	typingNewWord := line == "" || strings.HasSuffix(line, " ")

	if len(fields) == 0 || (len(fields) == 1 && !typingNewWord) {
		prefix := ""
		if len(fields) == 1 {
			prefix = fields[0]
		}

		return r.Candidates(prefix), true
	}

	command, found := r.Lookup(fields[0])
	if !found || command.Complete == nil {
		return nil, false
	}

	args := fields[1:]
	if typingNewWord {
		args = append(args, "")
	}

	return command.Complete(m, args), true
}

// CommonPrefix returns the longest prefix every candidate shares, which is how
// far one tab press can fill in without choosing for the user.
func CommonPrefix(candidates []Candidate) string {
	if len(candidates) == 0 {
		return ""
	}

	prefix := candidates[0].Name
	for _, candidate := range candidates[1:] {
		for !strings.HasPrefix(candidate.Name, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}

	return prefix
}

// ReplaceLastWord swaps the word being typed for word, keeping everything
// before it. A line ending in a space is opening a new word rather than
// extending the last one.
func ReplaceLastWord(line, word string) string {
	if strings.HasSuffix(line, " ") || line == "" {
		return line + word
	}

	idx := strings.LastIndex(line, " ")
	if idx < 0 {
		return word
	}

	return line[:idx+1] + word
}
