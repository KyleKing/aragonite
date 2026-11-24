// Package palette is the one place a reader finds anything a screen can do: a command palette on
// `:` that searches and runs, and the same list in browse mode on `?`, the legend, where pressing
// a key explains it instead of running it, `/` filters, and Enter runs. Both read one Entry list,
// built from a keys.Keymap and its Gate or from a legend tree, so the palette, the legend, and the
// footer can never disagree about what a key does or why it does nothing here.
//
// It draws in two layouts from one Model: a drawer on the right with an explainer pane under the
// list (what the selected entry runs, its flags, related keys, and the reader's own use of it), and
// below 100 columns a strip over the bottom rows that keeps the screen above it in view.
//
// Ranking puts what is available here first, then what was run recently, then what is off here
// with its reason. Off entries are never hidden, because seeing that something exists but needs X
// is how a reader learns X.
package palette

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"
)

// Entry is one thing the palette lists: an action, the key that presses it, and what a reader
// needs to learn it.
type Entry struct {
	// Recent is when the action last ran through any path, zero when never or unknown.
	Recent time.Time
	// Action is what RunMsg names when the entry runs. An entry built from a legend tree names its
	// key sequence, which the tool replays.
	Action string
	// Key is the label drawn in the key column: "s", "]h", "ctrl+d", "j/k".
	Key string
	// What is what the action does, as a legend says it.
	What string
	// Group is the legend heading the entry is filed under.
	Group string
	// Cmd is the command the action runs, as a reader would type it, so the palette teaches the
	// command line: "jj squash --from <rev> --into <dest>".
	Cmd string
	// Why is the reason an Off entry does nothing here.
	Why string
	// Flags summarizes the flags the action takes: "-p siblings  -A/-B placement".
	Flags string
	// Keys are the sequences that press it, spelled as keys.Binding spells them ("] h"), which is
	// how browse mode answers a key press with this entry.
	Keys []string
	// Also are words a search finds the entry by besides its own, git vocabulary included:
	// "amend", "fixup", "stash".
	Also []string
	// Prose is what the explainer pane says about the action at length.
	Prose []string
	// Related are keys worth knowing beside this one, as a reader would read them: "]h hunk".
	Related []string
	// Count is how many times the action ran, read only when Counted is set.
	Count int
	// Counted is whether Count is known, which it is only while a usage log is on.
	Counted bool
	// Off marks an entry that cannot run where the cursor is.
	Off bool
}

// About is what a tool knows about an action beyond its binding, for the palette to teach with.
type About struct {
	Cmd     string
	Flags   string
	Also    []string
	Prose   []string
	Related []string
}

// FromKeymap is every binding the keymap has a key for on screen ctx, in the keymap's order, each
// marked Off with its reason where gate refuses it and taught from about where the tool says more.
// Palette-only actions with no key are added by the caller as entries of their own.
func FromKeymap(k *keys.Keymap, ctx keys.Context, gate keys.Gate, about map[string]About) []Entry {
	bound := k.Bound(ctx)
	out := make([]Entry, 0, len(bound))

	for i := range bound {
		b := &bound[i]
		a := about[b.Action]
		avail := gate.Check(b.Action)

		out = append(out, Entry{
			Action:  b.Action,
			Key:     b.Labels(),
			What:    b.What,
			Group:   b.Group,
			Keys:    slices.Clone(b.Keys),
			Cmd:     a.Cmd,
			Flags:   a.Flags,
			Also:    a.Also,
			Prose:   a.Prose,
			Related: a.Related,
			Off:     !avail.OK,
			Why:     avail.Why,
		})
	}

	return out
}

// FromLegend flattens a legend tree, the shape keyhint.At walks, into entries: a heading names
// the group of the rows under it, a row with Kids is a prefix whose page is walked with its key
// in front, and a row with no key is prose the group's entries carry into the explainer pane. A
// prefix marked Off marks its whole page Off with its reason, since none of its keys can complete
// here. A row keyed "j/k" answers either half. Each entry's Action is its key sequence, which a tool with
// no action names replays.
func FromLegend(tree []keyhint.Hint) []Entry {
	return legend(nil, tree, nil, "")
}

func legend(out []Entry, page []keyhint.Hint, path []string, group string) []Entry {
	start := len(out)
	prose := map[string][]string{}

	for _, h := range page {
		switch {
		case h.Head:
			group = h.What
		case h.Key == "":
			prose[group] = append(prose[group], h.What)
		case len(h.Kids) > 0:
			from := len(out)
			out = legend(out, h.Kids, append(slices.Clone(path), h.Key), h.What)

			// A chord that is off here takes its objects with it, and they say its reason.
			for i := from; h.Off && i < len(out); i++ {
				out[i].Off, out[i].Why = true, h.Why
			}
		default:
			out = append(out, legendEntry(h, path, group))
		}
	}

	for i := start; i < len(out); i++ {
		if len(out[i].Prose) == 0 {
			out[i].Prose = prose[out[i].Group]
		}
	}

	return out
}

func legendEntry(h keyhint.Hint, path []string, group string) Entry {
	labels := Sequences(h.Key)
	seqs := make([]string, 0, len(labels))

	for _, seq := range labels {
		seqs = append(seqs, strings.Join(append(slices.Clone(path), seq), " "))
	}

	return Entry{
		Action: seqs[0],
		Key:    strings.Join(path, "") + h.Key,
		What:   h.What,
		Group:  group,
		Keys:   seqs,
		Off:    h.Off,
		Why:    h.Why,
	}
}

// named are the keys Bubble Tea spells as a word, which a legend label never means letter by
// letter.
var named = map[string]bool{
	keyEsc: true, keyTab: true, "space": true, keyEnter: true, "up": true, "down": true, "left": true,
	"right": true, "home": true, keyEnd: true, "pgup": true, "pgdown": true, "backspace": true,
	"delete": true, "insert": true,
}

// Sequences reads a legend's key label as the sequences it names, spelled as keys.Binding spells
// them: "j/k" is j or k, "ctrl+d/u" is ctrl+d or ctrl+u (the modifier carries to a bare letter
// after it), "gg/G" is g then g, or G, and "+/-" is + or -. A named key ("esc", "space") and a
// modified one stay whole.
func Sequences(label string) []string {
	parts := strings.Split(label, "/")
	if label == "/" || strings.HasSuffix(label, "//") {
		parts = []string{"/"}
	}

	mod := ""
	out := make([]string, 0, len(parts))

	for i, p := range parts {
		switch {
		case p == "":
			continue
		case strings.Contains(p, "+") && len(p) > 1:
			mod = p[:strings.LastIndex(p, "+")+1]
		case i > 0 && mod != "" && utf8.RuneCountInString(p) == 1:
			p = mod + p
		case !named[p] && utf8.RuneCountInString(p) > 1 && !strings.Contains(p, " "):
			p = strings.Join(strings.Split(p, ""), " ")
		}

		out = append(out, p)
	}

	return out
}

// Seen is what the palette reads about past use, satisfied by a coach.Ledger and by
// keyusage.Usage.Actions alike.
type Seen interface {
	Last(action string) time.Time
	Count(action string) int
}

// WithSeen fills each entry's Recent from seen, and Count where counted is set, which is the case
// only for a source that counts every run (a usage log), not a recent-commands file.
func WithSeen(entries []Entry, seen Seen, counted bool) []Entry {
	if seen == nil {
		return entries
	}

	out := slices.Clone(entries)

	for i := range out {
		if last := seen.Last(out[i].Action); last.After(out[i].Recent) {
			out[i].Recent = last
		}

		if counted {
			out[i].Count = max(out[i].Count, seen.Count(out[i].Action))
			out[i].Counted = true
		}
	}

	return out
}

// Describe is the entry a key press names, which is what browse mode answers instead of running
// it: the entry whose sequences include key (spelled as keys.Binding spells a sequence, "] h"),
// false when none does.
func Describe(entries []Entry, key string) (Entry, bool) {
	want := strings.Fields(key)

	for i := range entries {
		for _, seq := range entries[i].Keys {
			if slices.Equal(strings.Fields(seq), want) || (seq == "space" && key == " ") {
				return entries[i], true
			}
		}
	}

	return Entry{}, false
}

// under is the entries some sequence of which starts with prefix and runs past it, which is what
// a waiting prefix narrows browse mode to.
func under(entries []Entry, prefix []string) []Entry {
	var out []Entry

	for i := range entries {
		for _, seq := range entries[i].Keys {
			ks := strings.Fields(seq)
			if len(ks) > len(prefix) && slices.Equal(ks[:len(prefix)], prefix) {
				out = append(out, entries[i])

				break
			}
		}
	}

	return out
}
