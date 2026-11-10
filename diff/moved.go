package diff

import "strings"

// Move is where the other end of a moved line sits: the line it was taken from,
// for one the change adds, or the line it went to, for one it removes.
type Move struct {
	Path string
	Line int
	// First marks the run's opening line, and Lines is how long the run is.
	First bool
	Lines int
}

// moveFloor is the default MinRun: the fewest meaningful lines a run needs before
// it reads as moved, since two matching lines are as often a coincidence as a move.
const moveFloor = 3

// moveTextFloor is the default TextFloor, the shortest trimmed text that counts as meaningful.
const moveTextFloor = 4

// SeqLine names one line: an index into the sequences passed and a line index within it.
type SeqLine struct{ Seq, Line int }

// MoveOptions tunes MatchMoves. Zero values take the defaults noted per field.
type MoveOptions struct {
	// Barred reports a removed and an added line that must not match, such as two
	// lines of one hunk, which an edit in place would otherwise read as a move. Nil bars none.
	Barred func(removed, added SeqLine) bool
	// MinRun is the fewest meaningful lines a run needs to count as a move; default 3.
	MinRun int
	// TextFloor is the fewest trimmed bytes a line needs to count as meaningful; default 4.
	TextFloor int
}

// MatchMoves pairs each removed line with the added line it moved to. A run of
// consecutive lines counts when each line equals its partner on trimmed text
// and the run holds MinRun meaningful lines. Each added line starts the longest
// such run open to it, in sequence and line order, so each line is matched at
// most once. A tie goes to the earliest removed line.
func MatchMoves(removed, added [][]string, opt MoveOptions) map[SeqLine]SeqLine {
	if opt.MinRun == 0 {
		opt.MinRun = moveFloor
	}

	if opt.TextFloor == 0 {
		opt.TextFloor = moveTextFloor
	}

	m := &moveMatcher{
		removed: removed, added: added, opt: opt,
		starts:  map[string][]SeqLine{},
		matched: map[SeqLine]SeqLine{},
	}

	for s, lines := range removed {
		for i, text := range lines {
			k := moveKey(text)
			m.starts[k] = append(m.starts[k], SeqLine{s, i})
		}
	}

	for s, lines := range added {
		m.scan(s, lines)
	}

	return m.matched
}

// Moved is both ends of every run removed in one hunk and added in another,
// matched without indent. The same text taken out and put back in one hunk is an edit.
func (d *Diff) Moved() map[LineRef]Move {
	gone, came := d.changedRuns(KindRemove), d.changedRuns(KindAdd)
	pairs := MatchMoves(texts(gone), texts(came), MoveOptions{
		Barred: func(r, a SeqLine) bool { return gone[r.Seq][r.Line].line.Hunk == came[a.Seq][a.Line].line.Hunk },
	})

	out := map[LineRef]Move{}

	for r, a := range pairs {
		if prev, ok := pairs[SeqLine{r.Seq, r.Line - 1}]; ok && prev == (SeqLine{a.Seq, a.Line - 1}) {
			continue
		}

		n := runLength(pairs, r, a)

		for k := range n {
			g, c := gone[r.Seq][r.Line+k], came[a.Seq][a.Line+k]
			out[refOf(c.path, c.line)] = Move{Path: g.path, Line: g.line.Old, First: k == 0, Lines: n}
			out[refOf(g.path, g.line)] = Move{Path: c.path, Line: c.line.New, First: k == 0, Lines: n}
		}
	}

	return out
}

// moveStartCap bounds how many removed runs one added line checks against.
// Past it the candidates are duplicates of the same few lines — generated
// code and blank-ish lines — where checking more finds no better move but
// costs quadratically in the duplicates.
const moveStartCap = 32

type moveMatcher struct {
	starts  map[string][]SeqLine
	matched map[SeqLine]SeqLine
	removed [][]string
	added   [][]string
	opt     MoveOptions
}

func (m *moveMatcher) scan(s int, lines []string) {
	for i := 0; i < len(lines); {
		n, from := m.longestRun(SeqLine{s, i})
		if n == 0 {
			i++

			continue
		}

		for k := range n {
			m.matched[SeqLine{from.Seq, from.Line + k}] = SeqLine{s, i + k}
		}

		i += n
	}
}

// longestRun is the longest run starting at an added line that some unmatched
// removed run repeats line for line, and where that removed run starts.
func (m *moveMatcher) longestRun(at SeqLine) (int, SeqLine) {
	text := m.added[at.Seq][at.Line]
	if !meaningful(text, m.opt.TextFloor) {
		return 0, SeqLine{}
	}

	starts := m.starts[moveKey(text)]
	best, from := 0, SeqLine{}

	for _, s := range starts[:min(len(starts), moveStartCap)] {
		if m.opt.Barred != nil && m.opt.Barred(s, at) {
			continue
		}

		n, meant := m.run(at, s)
		if meant >= m.opt.MinRun && n > best {
			best, from = n, s
		}
	}

	return best, from
}

// run is how many leading lines from at repeat the unmatched lines from s, trailing
// blanks dropped, and how many of those are meaningful.
func (m *moveMatcher) run(at, s SeqLine) (int, int) {
	came, gone := m.added[at.Seq][at.Line:], m.removed[s.Seq][s.Line:]
	n, meant := 0, 0

	for n < len(came) && n < len(gone) {
		if _, taken := m.matched[SeqLine{s.Seq, s.Line + n}]; taken || moveKey(came[n]) != moveKey(gone[n]) {
			break
		}

		if meaningful(came[n], m.opt.TextFloor) {
			meant++
		}

		n++
	}

	for n > 0 && moveKey(came[n-1]) == "" {
		n--
	}

	return n, meant
}

// placed is a changed line with the path it sits in.
type placed struct {
	path string
	line Line
}

// changedRuns groups each file's lines of one kind into runs of consecutive lines.
// A context line ends a run, so a move never matches across unchanged text.
func (d *Diff) changedRuns(kind byte) [][]placed {
	var runs [][]placed

	for i := range d.Files {
		path := pathOf(&d.Files[i])

		var run []placed

		for _, l := range d.Files[i].Lines {
			if l.Kind != kind {
				if len(run) > 0 {
					runs = append(runs, run)
				}

				run = nil

				continue
			}

			run = append(run, placed{path, l})
		}

		if len(run) > 0 {
			runs = append(runs, run)
		}
	}

	return runs
}

func texts(runs [][]placed) [][]string {
	out := make([][]string, len(runs))

	for i, run := range runs {
		for _, p := range run {
			out[i] = append(out[i], p.line.Text)
		}
	}

	return out
}

// runLength is how many lines from r and a pair up in step.
func runLength(pairs map[SeqLine]SeqLine, r, a SeqLine) int {
	n := 0

	for {
		next, ok := pairs[SeqLine{r.Seq, r.Line + n}]
		if !ok || next != (SeqLine{a.Seq, a.Line + n}) {
			return n
		}

		n++
	}
}

func moveKey(text string) string { return strings.TrimSpace(text) }

// meaningful reports whether a line carries enough to match on, so a run does
// not open on a blank or a lone brace.
func meaningful(text string, floor int) bool { return len(moveKey(text)) >= floor }
