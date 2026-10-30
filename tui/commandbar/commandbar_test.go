package commandbar_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/aragonite/tui/commandbar"
)

type screen struct{ names []string }

func newRegistry() commandbar.Registry[screen] {
	return commandbar.NewRegistry(
		commandbar.Command[screen]{Name: "reset", Description: "reset"},
		commandbar.Command[screen]{Name: "chain", Description: "chain"},
		commandbar.Command[screen]{Name: "check", Description: "check"},
		commandbar.Command[screen]{
			Name:        "workflow",
			Description: "workflow",
			Complete: func(m screen, args []string) []commandbar.Candidate {
				var out []commandbar.Candidate

				for _, name := range m.names {
					if strings.HasPrefix(name, args[len(args)-1]) {
						out = append(out, commandbar.Candidate{Name: name})
					}
				}

				return out
			},
		},
		commandbar.Command[screen]{Name: "watch", Description: "watch"},
	)
}

// An ambiguous prefix resolves to nothing, so `:c` never runs whichever
// command sorts first.
func TestRegistryLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"exact", "reset", "reset"},
		{"unique prefix", "r", "reset"},
		{"ambiguous prefix", "ch", ""},
		{"exact wins over prefix", "chain", "chain"},
		{"unknown", "zzz", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			command, found := newRegistry().Lookup(tt.query)

			if tt.want == "" {
				if found {
					t.Errorf("Lookup(%q) resolved to %q, want nothing", tt.query, command.Name)
				}

				return
			}

			if !found || command.Name != tt.want {
				t.Errorf("Lookup(%q) = %q (%v), want %q", tt.query, command.Name, found, tt.want)
			}
		})
	}
}

func TestCommonPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		want  string
		names []string
	}{
		{"none", "", nil},
		{"one", "chain", []string{"chain"}},
		{"shared", "ch", []string{"chain", "check"}},
		{"nothing shared", "", []string{"chain", "reset"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			candidates := make([]commandbar.Candidate, len(tt.names))
			for i, name := range tt.names {
				candidates[i] = commandbar.Candidate{Name: name}
			}

			if got := commandbar.CommonPrefix(candidates); got != tt.want {
				t.Errorf("CommonPrefix(%v) = %q, want %q", tt.names, got, tt.want)
			}
		})
	}
}

func TestReplaceLastWord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, line, word, want string
	}{
		{"empty line", "", "reset", "reset"},
		{"first word", "re", "reset", "reset"},
		{"later word", "workflow ci", "ci.yml", "workflow ci.yml"},
		{"opening a new word", "workflow ", "ci.yml", "workflow ci.yml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := commandbar.ReplaceLastWord(tt.line, tt.word); got != tt.want {
				t.Errorf("ReplaceLastWord(%q, %q) = %q, want %q", tt.line, tt.word, got, tt.want)
			}
		})
	}
}

// Completing an argument asks the command; a command with nothing to complete
// says so rather than offering command names again.
func TestCompletions(t *testing.T) {
	t.Parallel()

	m := screen{names: []string{"ci.yml", "deploy.yml"}}

	tests := []struct {
		name        string
		line        string
		want        []string
		completable bool
	}{
		{"empty line offers every command", "", []string{"chain", "check", "reset", "watch", "workflow"}, true},
		{"command name prefix", "wa", []string{"watch"}, true},
		{"ambiguous command prefix", "ch", []string{"chain", "check"}, true},
		{"argument of a completing command", "workflow ", []string{"ci.yml", "deploy.yml"}, true},
		{"argument prefix", "workflow dep", []string{"deploy.yml"}, true},
		{"command taking no argument", "watch ", nil, false},
		{"unknown command", "nope ", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			candidates, completable := newRegistry().Completions(m, tt.line)
			if completable != tt.completable {
				t.Fatalf("Completions(%q) completable = %v, want %v", tt.line, completable, tt.completable)
			}

			var got []string
			for _, c := range candidates {
				got = append(got, c.Name)
			}

			if !slices.Equal(got, tt.want) {
				t.Errorf("Completions(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// The listing is the reference for a caller with no help modal, and it comes
// back sorted whatever order the commands were registered in.
func TestListing(t *testing.T) {
	t.Parallel()

	registry := commandbar.NewRegistry(
		commandbar.Command[screen]{Name: "b", Description: "second"},
		commandbar.Command[screen]{Name: "a", Description: "first"},
	)

	if got, want := registry.Listing(), "a\tfirst\nb\tsecond"; got != want {
		t.Errorf("Listing() = %q, want %q", got, want)
	}
}

// Run takes and returns the caller's own model type.
func TestCommandRunReturnsTheCallersModel(t *testing.T) {
	t.Parallel()

	registry := commandbar.NewRegistry(commandbar.Command[screen]{
		Name: "add",
		Run: func(m screen, args []string) (screen, tea.Cmd) {
			m.names = append(m.names, args...)

			return m, nil
		},
	})

	command, found := registry.Lookup("a")
	if !found {
		t.Fatal("unique prefix did not resolve")
	}

	after, _ := command.Run(screen{}, []string{"x", "y"})
	if !slices.Equal(after.names, []string{"x", "y"}) {
		t.Errorf("Run returned %v", after.names)
	}
}
