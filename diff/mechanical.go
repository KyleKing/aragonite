package diff

// HunkRef names one hunk of a diff by its file and its Line.Hunk number.
type HunkRef struct {
	Path string
	Hunk int
}

// MechanicalHunks returns the hunks one reason covers whole, "reindent only" or
// "whitespace only". A hunk with any real edit or moved line stays out.
func (d *Diff) MechanicalHunks() map[HunkRef]string {
	// One pass gathers every hunk's tally and its squeezed sides, so judging a
	// hunk whitespace-only does not walk the whole file again per hunk.
	type tally struct {
		added, removed []string
		n, indent      int
	}

	tallies := map[HunkRef]*tally{}
	reindented := d.Reindented()

	for i := range d.Files {
		p := pathOf(&d.Files[i])

		for _, l := range d.Files[i].Lines {
			if l.Kind == KindContext || l.Hunk == 0 {
				continue
			}

			at := HunkRef{p, l.Hunk}
			t := tallies[at]

			if t == nil {
				t = &tally{}
				tallies[at] = t
			}

			t.n++

			if reindented[refOf(p, l)] {
				t.indent++
			}

			if l.Kind == KindAdd {
				t.added = append(t.added, squeeze(l.Text))
			} else {
				t.removed = append(t.removed, squeeze(l.Text))
			}
		}
	}

	out := map[HunkRef]string{}

	for at, t := range tallies {
		switch {
		case t.indent == t.n:
			out[at] = "reindent only"
		case sameWords(t.added, t.removed):
			out[at] = "whitespace only"
		}
	}

	return out
}
