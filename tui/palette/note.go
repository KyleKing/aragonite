package palette

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/table"
)

// noteLabel is the width of the label column in a note: "runs", "flags", "also", "you".
const noteLabel = 7

// Note is what a reader is told about one entry, width cells wide: a labeled rule naming its group,
// primary key, and description (so a key under a chord never reads as a root key), its prose, why
// it is off here, every key that presses it where there is more than one, the command it runs, its
// flags, the keys worth knowing beside it, and the reader's own use of it as of now. It is the
// explainer pane under the drawer's list, the answer browse mode gives for a key press, and what a
// tool draws in its own status area for the same question.
func Note(s skin.Skin, e Entry, width int, now time.Time) []string {
	if width < 1 {
		return nil
	}

	keys := KeyLabels(e.Key)
	name := strings.TrimSpace(keys[0] + "  " + e.What)
	if e.Group != "" {
		name = e.Group + " · " + name
	}

	out := []string{s.Rule(name, "", width)}

	for _, p := range e.Prose {
		for _, l := range Wrap(p, width) {
			out = append(out, s.Body.Render(l))
		}
	}

	fact := func(label, value string, face lipgloss.Style) {
		if value == "" {
			return
		}

		lead := table.Pad(label, noteLabel, table.AlignLeft)
		for i, l := range Wrap(value, max(1, width-noteLabel)) {
			if i > 0 {
				lead = strings.Repeat(" ", noteLabel)
			}

			out = append(out, s.Muted.Render(lead)+face.Render(l))
		}
	}

	if e.Off {
		fact("off", e.Why, s.Warning)
	}

	if len(keys) > 1 {
		fact("keys", strings.Join(keys, "  "), s.Key)
	}

	fact("runs", e.Cmd, s.Body)
	fact("flags", e.Flags, s.Body)
	fact("also", strings.Join(e.Related, "  "), s.Body)
	fact("you", used(e, now), s.Muted)

	return out
}

// used is the reader's own history with the entry, as far as it is known.
func used(e Entry, now time.Time) string {
	var parts []string

	if e.Counted {
		if e.Count == 0 {
			parts = append(parts, "never used")
		} else {
			parts = append(parts, "used "+strconv.Itoa(e.Count)+"×")
		}
	}

	if !e.Recent.IsZero() {
		parts = append(parts, "last run "+Ago(now.Sub(e.Recent)))
	}

	return strings.Join(parts, " · ")
}

// Wrap breaks prose at word boundaries into lines at most width cells wide, cutting a single word
// that is wider still.
func Wrap(text string, width int) []string {
	var out []string

	for _, w := range strings.Fields(text) {
		if n := len(out); n > 0 && lipgloss.Width(out[n-1])+1+lipgloss.Width(w) <= width {
			out[n-1] += " " + w
		} else {
			out = append(out, table.Truncate(w, width))
		}
	}

	return out
}
