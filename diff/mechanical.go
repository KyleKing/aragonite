package diff

// HunkRef names one hunk of a diff by its file and its Line.Hunk number.
type HunkRef struct {
	Path string
	Hunk int
}

// MechanicalHunks returns the hunks one reason covers whole, "reindent only" or
// "whitespace only". A hunk with any real edit or moved line stays out.
func (d *Diff) MechanicalHunks() map[HunkRef]string {
	// A reindented line's mate says the same thing in the same hunk, so a
	// reindent-only hunk is whitespace-only too. Only those need the pairing,
	// which is the expensive read, and a diff with none skips it.
	out := map[HunkRef]string{}

	var reindented map[LineRef]bool

	for at, h := range d.changedHunks() {
		if !sameWords(h.added, h.removed) {
			continue
		}

		if reindented == nil {
			reindented = d.Reindented()
		}

		out[at] = "whitespace only"

		if allReindented(reindented, at.Path, h.lines) {
			out[at] = "reindent only"
		}
	}

	return out
}

// changedHunk is one hunk's changed lines and each side squeezed of its spaces.
type changedHunk struct {
	added, removed []string
	lines          []Line
}

// changedHunks gathers every hunk's changed lines in one pass, so judging a hunk
// does not walk the whole file again per hunk.
func (d *Diff) changedHunks() map[HunkRef]*changedHunk {
	out := map[HunkRef]*changedHunk{}

	for i := range d.Files {
		p := pathOf(&d.Files[i])

		for _, l := range d.Files[i].Lines {
			if l.Kind == KindContext || l.Hunk == 0 {
				continue
			}

			at := HunkRef{p, l.Hunk}
			h := out[at]

			if h == nil {
				h = &changedHunk{}
				out[at] = h
			}

			h.lines = append(h.lines, l)

			if l.Kind == KindAdd {
				h.added = append(h.added, squeeze(l.Text))
			} else {
				h.removed = append(h.removed, squeeze(l.Text))
			}
		}
	}

	return out
}

func allReindented(reindented map[LineRef]bool, path string, lines []Line) bool {
	for _, l := range lines {
		if !reindented[refOf(path, l)] {
			return false
		}
	}

	return true
}
