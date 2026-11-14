package keyusage

import (
	"strings"
)

// CLI is the context command-line invocations are logged and reported under.
const CLI = "cli"

// Command is one command a tool's CLI accepts: its name, what it does, the
// flags it takes as typed ("--json", "-n"), and its subcommands.
type Command struct {
	Name  string
	What  string
	Flags []string
	Subs  []Command
}

// Invoke records which command and flags args ran, as `Invoke(root, os.Args[1:])`.
// Only names root declares are recorded: positional arguments and flag values
// are dropped, since they carry paths, repositories, and tokens.
func (r *Recorder) Invoke(root Command, args []string) {
	if r == nil {
		return
	}

	r.put(Entry{S: CLI, E: CLI, W: invocation(root, args)})
}

// invocation is the command path followed by each flag's name, such as
// "tool pr view --web". Subcommands are read until the first bare word that is
// not one, which is a positional argument or a flag's value.
func invocation(root Command, args []string) string {
	path, flags := []string{root.Name}, []string(nil)
	cmd, walking := root, true

	for _, a := range args {
		if a == "--" {
			break
		}

		if strings.HasPrefix(a, "-") && a != "-" {
			name, _, _ := strings.Cut(a, "=")
			flags = append(flags, name)

			continue
		}

		sub, ok := subcommand(cmd, a)
		if !walking || !ok {
			walking = false

			continue
		}

		path = append(path, sub.Name)
		cmd = sub
	}

	return strings.Join(append(path, flags...), " ")
}

func subcommand(cmd Command, name string) (Command, bool) {
	for _, s := range cmd.Subs {
		if s.Name == name {
			return s, true
		}
	}

	return Command{}, false
}

// Commands is root's CLI as bindings for the CLI context: one per command path
// and one per flag on it ("tool pr view --web"), so Cold and Write report an
// unused flag the way they report an unused key.
func Commands(root Command) []Binding {
	return commandBindings(nil, "", root)
}

func commandBindings(out []Binding, prefix string, cmd Command) []Binding {
	path := strings.TrimSpace(prefix + " " + cmd.Name)
	out = append(out, Binding{Keys: []string{path}, What: cmd.What})

	for _, f := range cmd.Flags {
		out = append(out, Binding{Keys: []string{path + " " + f}, What: cmd.What})
	}

	for _, s := range cmd.Subs {
		out = commandBindings(out, path, s)
	}

	return out
}

// tallyCLI counts one invocation's command path and each of its flags, and
// misses a flag the command does not declare.
func (u *Usage) tallyCLI(e Entry, km Keymap) {
	bindings, ok := km[CLI]
	if !ok {
		u.Unmapped[CLI]++

		return
	}

	words := strings.Fields(e.W)
	path := words

	for i, w := range words {
		if strings.HasPrefix(w, "-") {
			path = words[:i]

			break
		}
	}

	cmd := strings.Join(path, " ")
	bump(u.hits, CLI, cmd, e.T)

	for _, f := range words[len(path):] {
		key := cmd + " " + f
		if find(bindings, key) != nil {
			bump(u.hits, CLI, key, e.T)
		} else {
			bump(u.misses, CLI, key, e.T)
		}
	}
}
