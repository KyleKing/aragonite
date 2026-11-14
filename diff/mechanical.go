package diff

// HunkRef names one hunk of a diff by its file and its Line.Hunk number.
type HunkRef struct {
	Path string
	Hunk int
}

// MechanicalHunks returns the hunks one reason covers whole, "reindent only" or
// "whitespace only". A hunk with any real edit or moved line stays out.
func (d *Diff) MechanicalHunks() map[HunkRef]string {
	type tally struct{ n, indent int }

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
		}
	}

	out := map[HunkRef]string{}

	for at, t := range tallies {
		switch {
		case t.indent == t.n:
			out[at] = "reindent only"
		case d.WhitespaceOnly(at.Path, at.Hunk):
			out[at] = "whitespace only"
		}
	}

	return out
}
