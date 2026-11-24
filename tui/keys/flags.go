package keys

import "strings"

// Flag is one toggle a destination picker offers after the - prefix, spelled as jj's own short flag
// so the footer's live command line reads as what will run.
type Flag struct {
	// Letter is the key after -, case-exact: -b is branch and -B is insert-before.
	Letter string
	// Arg is the flag jj receives: "--keep-emptied", "-B".
	Arg string
	// What is the flag as a legend says it.
	What string
	// Radio names the group of flags only one of which is on, empty for a plain toggle. The one
	// marked Default is on until another in its group is chosen.
	Radio string
	// Default is on when the picker opens.
	Default bool
}

// Verbs whose pickers take flags, as Flags names them.
const (
	VerbRebase    = "rebase"
	VerbSquash    = "squash"
	VerbDuplicate = "duplicate"
	VerbNew       = "new"
	VerbRestore   = "restore"
	VerbCompare   = "compare"
	VerbSplit     = "split"
)

// Radio group names.
const (
	RadioScope     = "scope"
	RadioPlacement = "placement"
)

// Flags is the - page of a verb's picker, nil for a verb with none. The footer, the palette, the
// help page, and the command a picker runs all derive from it, so a flag jj renames is one row here.
func Flags(verb string) []Flag {
	placement := func(def string) []Flag {
		out := []Flag{
			{Letter: "o", Arg: "-o", What: "onto", Radio: RadioPlacement},
			{Letter: "A", Arg: "-A", What: "insert after", Radio: RadioPlacement},
			{Letter: "B", Arg: "-B", What: "insert before", Radio: RadioPlacement},
		}

		for i := range out {
			out[i].Default = out[i].Letter == def
		}

		return out
	}

	switch verb {
	case VerbRebase:
		return append(append([]Flag{
			{Letter: "r", Arg: "-r", What: "the revision alone", Radio: RadioScope, Default: true},
			{Letter: "s", Arg: "-s", What: "the revision and its descendants", Radio: RadioScope},
			{Letter: "b", Arg: "-b", What: "the whole branch", Radio: RadioScope},
		}, placement("o")...),
			Flag{Letter: "e", Arg: "--skip-emptied", What: "abandon commits left empty"},
			Flag{Letter: "d", Arg: "--keep-divergent", What: "keep divergent commits"},
			Flag{Letter: "p", Arg: "--simplify-parents", What: "simplify parents"},
		)
	case VerbSquash:
		return append([]Flag{
			{Letter: "t", Arg: "--into", What: "into", Radio: RadioPlacement, Default: true},
			{Letter: "o", Arg: "-o", What: "into a new commit onto", Radio: RadioPlacement},
			{Letter: "A", Arg: "-A", What: "into a new commit after", Radio: RadioPlacement},
			{Letter: "B", Arg: "-B", What: "into a new commit before", Radio: RadioPlacement},
		},
			Flag{Letter: "u", Arg: "--use-destination-message", What: "keep the destination's message"},
			Flag{Letter: "m", Arg: "-m", What: "write the message"},
			Flag{Letter: "k", Arg: "--keep-emptied", What: "keep the source if it empties"},
			Flag{Letter: "i", Arg: "-i", What: "choose hunks"},
		)
	case VerbDuplicate:
		return placement("o")
	case VerbNew:
		return append(placement("o"),
			Flag{Letter: "n", Arg: "--no-edit", What: "stay where you are"},
			Flag{Letter: "m", Arg: "-m", What: "write the message"},
		)
	case VerbRestore:
		return []Flag{
			{Letter: "i", Arg: "-i", What: "choose hunks"},
			{Letter: "d", Arg: "--restore-descendants", What: "keep descendants' content"},
		}
	case VerbCompare:
		return []Flag{
			{Letter: "i", Arg: "interdiff", What: "interdiff instead of diff"},
			{Letter: "r", Arg: "reverse", What: "swap the sides"},
		}
	case VerbSplit:
		return append(placement(""), Flag{Letter: "p", Arg: "--parallel", What: "siblings instead of a chain"})
	}

	return nil
}

// FlagBindings is a verb's flags as picker bindings, "- k" pressing "squash-flag-k", so a picker's
// which-key panel and legend come from the same table as its command line.
func FlagBindings(verb string) []Binding {
	flags := Flags(verb)
	out := make([]Binding, 0, len(flags))

	for _, f := range flags {
		out = append(out, Binding{
			Action: FlagAction(verb, f.Letter), Keys: []string{"- " + f.Letter},
			What: f.What + " (" + f.Arg + ")", Short: f.Arg, Group: "Flags", Contexts: Picker,
		})
	}

	return out
}

// FlagAction is the action a flag's binding presses.
func FlagAction(verb, letter string) string { return verb + "-flag-" + letter }

// FlagSet is which of a verb's flags are on. Start one with NewFlagSet.
type FlagSet struct {
	on   map[string]bool
	verb string
}

// NewFlagSet starts verb's flags at their defaults.
func NewFlagSet(verb string) FlagSet {
	s := FlagSet{verb: verb, on: map[string]bool{}}

	for _, f := range Flags(verb) {
		if f.Default {
			s.on[f.Letter] = true
		}
	}

	return s
}

// Toggle flips the flag with letter, false when the verb has none. A radio flag is set rather than
// flipped, turning the others in its group off, since a radio group always holds one choice.
func (s *FlagSet) Toggle(letter string) bool {
	flags := Flags(s.verb)
	if s.on == nil {
		s.on = map[string]bool{}
	}

	for _, f := range flags {
		if f.Letter != letter {
			continue
		}

		if f.Radio == "" {
			s.on[letter] = !s.on[letter]

			return true
		}

		for _, o := range flags {
			if o.Radio == f.Radio {
				delete(s.on, o.Letter)
			}
		}

		s.on[letter] = true

		return true
	}

	return false
}

// On reports whether the flag with letter is on.
func (s FlagSet) On(letter string) bool { return s.on[letter] }

// Args is the jj arguments for the plain toggles that are on, in table order. Radio flags are left
// out because each one carries the revisions the picker chose (-o, -B, --into, -s), as is -m, which
// carries a message from a prompt, and so is any flag that is not jj's own (compare's interdiff).
func (s FlagSet) Args() []string {
	var out []string

	for _, f := range Flags(s.verb) {
		if !s.on[f.Letter] || f.Radio != "" || f.Arg == "-m" || !strings.HasPrefix(f.Arg, "-") {
			continue
		}

		out = append(out, f.Arg)
	}

	return out
}
