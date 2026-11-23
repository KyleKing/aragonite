package diff

import (
	"slices"
	"strconv"
	"strings"
)

// Span is a half-open byte range of a line's text.
type Span struct {
	From int
	To   int
}

// LineRef names one line of a diff. The pair of numbers is enough on its own: a
// removed line carries only Old and an added line only New, so no two lines of
// one file share a ref.
type LineRef struct {
	Path string
	Kind byte
	Old  int
	New  int
}

// Refined is the runs of each changed line that its partner does not carry. A
// line absent from it changed as a whole, which is what the line's own color
// already says.
type Refined map[LineRef][]Span

// Refine pairs the removed and added lines of every hunk and reports which runs
// of each differ, so a one-word edit reads as one word rather than as two
// entirely changed lines.
//
// Pairing is by best match inside a change block, which is a run of removals
// followed by a run of additions: each removal answers the addition most like
// it, never crossing another pair. An unpaired line is left whole, and so is a
// pair with too little in common, since marking most of a rewritten line is
// noise rather than information.
func (d *Diff) Refine() Refined { return d.pair().marks }

// Reindented is every changed line paired with one saying the same thing at
// another indent, which is a block gaining or losing a level around it.
func (d *Diff) Reindented() map[LineRef]bool { return d.pair().reindented }

// Paired is the pairing in one pass: the changed runs, the re-indented lines,
// the moved runs, and each paired line's mate, which Refine, Reindented,
// Moved, and Mates each answer alone. A caller wanting them all asks once,
// because the pairing is the expensive read a diff offers.
func (d *Diff) Paired() (Refined, map[LineRef]bool, map[LineRef]Move, map[LineRef]Line) {
	p := d.pair()

	return p.marks, p.reindented, p.moved, p.mate
}

// Mates is each paired line's partner on the other side, which is what an edit
// drawn as one line looks the removed text up with.
func (d *Diff) Mates() map[LineRef]Line { return d.pair().mate }

type pairing struct {
	marks      Refined
	reindented map[LineRef]bool
	// moved lines answer a run elsewhere, so they pair with nothing in their own block.
	moved map[LineRef]Move
	// mate is the line a changed line paired with on the other side, kept
	// rather than its ref so a caller need not find it in the diff again.
	mate map[LineRef]Line
}

func (d *Diff) pair() pairing {
	if d.paired == nil {
		out := pairing{
			marks: Refined{}, reindented: map[LineRef]bool{}, moved: d.Moved(),
			mate: map[LineRef]Line{},
		}

		for i := range d.Files {
			refineFile(out, pathOf(&d.Files[i]), d.Files[i].Lines)
		}

		d.paired = &out
	}

	return *d.paired
}

// blockCap bounds the line pairing, which is quadratic in a block's lines.
const blockCap = 160_000

// indentKey is a line's text without its indent, or a key nothing matches
// where that text is too short to mean anything.
func indentKey(text, side string, at int) string {
	if unimportant(text) {
		return side + strconv.Itoa(at) + "\x00"
	}

	return strings.TrimLeft(text, " \t")
}

// reindented reports two different lines that say the same thing at another indent.
func reindented(before, after string) bool {
	return before != after && !unimportant(before) &&
		strings.TrimLeft(before, " \t") == strings.TrimLeft(after, " \t")
}

// unimportant reports a line too short to anchor a pairing on its own: a lone
// `}`, `)`, `end`, or a blank. Any two of them look alike, so letting one decide
// a pair pulls the alignment away from the lines that carry the change.
func unimportant(text string) bool {
	return len(strings.TrimSpace(text)) < indentFloor
}

// indentFloor is the shortest trimmed text that anchors a pairing, so a lone
// brace or `)` does not pair across a block.
const indentFloor = 4

func refineOne(out pairing, path string, gone, came Line) {
	before, after, ok := mark(gone.Text, came.Text)
	if !ok {
		return
	}

	out.marks[refOf(path, gone)] = before
	out.marks[refOf(path, came)] = after
	out.mate[refOf(path, gone)], out.mate[refOf(path, came)] = came, gone
}

func refineFile(out pairing, path string, lines []Line) {
	var gone, came []Line

	flush := func() {
		refineBlock(out, path, gone, came)

		gone, came = nil, nil
	}

	for _, l := range lines {
		if _, ok := out.moved[refOf(path, l)]; ok {
			continue
		}

		switch l.Kind {
		case KindRemove:
			// An addition already seen closes the block, so what follows opens
			// a new one rather than pairing across an unrelated boundary.
			if len(came) > 0 {
				flush()
			}

			gone = append(gone, l)
		case KindAdd:
			came = append(came, l)
		default:
			flush()
		}
	}

	flush()
}

// refineBlock marks every pair alignBlock finds in one change block, and flags
// a pair saying the same thing at another indent as re-indented.
func refineBlock(out pairing, path string, gone, came []Line) {
	if len(gone) == 0 || len(came) == 0 {
		return
	}

	left, right := make([]string, len(gone)), make([]string, len(came))
	for i, l := range gone {
		left[i] = l.Text
	}

	for i, l := range came {
		right[i] = l.Text
	}

	for _, pair := range alignBlock(left, right) {
		g, c := gone[pair[0]], came[pair[1]]
		refineOne(out, path, g, c)

		if reindented(g.Text, c.Text) {
			out.reindented[refOf(path, g)], out.reindented[refOf(path, c)] = true, true
		}
	}
}

// alignBlock pairs a block's removals with its additions, as index pairs in
// ascending order that never cross. It runs in three tiers, each confined to
// the stretches the tiers before it left between their pairs:
//
//  1. Lines saying the same thing at another indent, so a line pushed under a
//     new `if` does not pair with the `if` by position.
//  2. The best match by shared tokens among lines long enough to mean
//     something, so a prose edit that merged two lines still pairs the line
//     it kept. Two lines that traded places are a move, which pair() has
//     already taken out of the block.
//  3. The same match with the short lines let in, only where an earlier pair
//     anchors the stretch. A lone `}` matches any other, so allowed to anchor
//     it would outweigh one real edit and push it off the alignment.
func alignBlock(gone, came []string) [][2]int {
	if len(gone) == 0 || len(came) == 0 {
		return nil
	}

	if len(gone)*len(came) > blockCap {
		// The match table is quadratic in the block, so past the cap the
		// positional pairing is what a huge generated block gets.
		out := make([][2]int, min(len(gone), len(came)))
		for at := range out {
			out[at] = [2]int{at, at}
		}

		return out
	}

	pairs := reindentPairs(gone, came)
	weights := score(gone, came)
	shortGone, shortCame := shortLines(gone), shortLines(came)

	pairs = between(pairs, len(gone), len(came), func(i, j int) float64 {
		if shortGone[i] || shortCame[j] {
			return 0
		}

		return weights[i][j]
	})

	if len(pairs) == 0 {
		return nil
	}

	return between(pairs, len(gone), len(came), func(i, j int) float64 { return weights[i][j] })
}

// reindentPairs is the longest run of pairs that match once their indent is
// set aside, leaving out the short lines that would match anything.
func reindentPairs(gone, came []string) [][2]int {
	left, right := make([]string, len(gone)), make([]string, len(came))
	for i, text := range gone {
		left[i] = indentKey(text, "-", i)
	}

	for i, text := range came {
		right[i] = indentKey(text, "+", i)
	}

	atLeft, atRight := common(left, right)
	out := make([][2]int, len(atLeft))

	for k := range atLeft {
		out[k] = [2]int{atLeft[k], atRight[k]}
	}

	return out
}

// shortLines flags each line unimportant reports.
func shortLines(lines []string) []bool {
	out := make([]bool, len(lines))
	for i, text := range lines {
		out[i] = unimportant(text)
	}

	return out
}

// between keeps the anchors and adds the best non-crossing pairs weight
// offers in each stretch of an n-by-m block they leave open, in ascending order.
func between(anchors [][2]int, n, m int, weight func(i, j int) float64) [][2]int {
	out := make([][2]int, 0, len(anchors))
	i, j := 0, 0

	for k := 0; k <= len(anchors); k++ {
		anchor := [2]int{n, m}
		if k < len(anchors) {
			anchor = anchors[k]
		}

		for _, pair := range matched(anchor[0]-i, anchor[1]-j, func(di, dj int) float64 {
			return weight(i+di, j+dj)
		}) {
			out = append(out, [2]int{i + pair[0], j + pair[1]})
		}

		if k < len(anchors) {
			out = append(out, anchor)
		}

		i, j = anchor[0]+1, anchor[1]+1
	}

	return out
}

// score weighs every removal against every addition on their shared tokens
// over the mean length, and floors out the pairs too unlike to mark. Two equal
// lines weigh fully, blank ones included, since they plainly sit opposite.
func score(gone, came []string) [][]float64 {
	left, right := make([]map[string]int, len(gone)), make([]map[string]int, len(came))
	for i, text := range gone {
		left[i] = bagOf(text)
	}

	for i, text := range came {
		right[i] = bagOf(text)
	}

	out := make([][]float64, len(gone))
	cells := make([]float64, len(gone)*len(came))

	for i := range gone {
		out[i] = cells[i*len(came) : (i+1)*len(came)]

		for j := range came {
			if gone[i] == came[j] {
				out[i][j] = 1

				continue
			}

			mean := float64(len(gone[i])+len(came[j])) / halves
			if mean > 0 {
				out[i][j] = float64(shared(left[i], right[j])) / mean
			}

			if out[i][j] < sameEnough {
				out[i][j] = 0
			}
		}
	}

	return out
}

// matched walks back the best non-crossing set of pairs an n-by-m weight
// table offers, in ascending order, which is a longest-subsequence alignment
// scored on lines instead of tokens. A pair weighing nothing is never kept.
func matched(n, m int, weight func(i, j int) float64) [][2]int {
	if n <= 0 || m <= 0 {
		return nil
	}

	cols := m + 1
	best := make([]float64, (n+1)*cols)

	at := func(i, j int) float64 { return best[i*cols+j] }
	set := func(i, j int, v float64) { best[i*cols+j] = v }

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			set(i, j, max(
				max(at(i-1, j), at(i, j-1)),
				at(i-1, j-1)+weight(i-1, j-1),
			))
		}
	}

	var out [][2]int

	for i, j := n, m; i > 0 && j > 0; {
		switch at(i, j) {
		case at(i-1, j):
			i--
		case at(i, j-1):
			j--
		default:
			if weight(i-1, j-1) > 0 {
				out = append(out, [2]int{i - 1, j - 1})
			}

			i--
			j--
		}
	}

	slices.Reverse(out)

	return out
}

// halves is the count a mean of two lengths divides by.
const halves = 2

// bagOf is a line's tokens as a multiset, which is what a match is weighed on
// before the costlier token-sequence check decides the pair.
func bagOf(text string) map[string]int {
	out := map[string]int{}
	for _, s := range tokens(text) {
		if strings.TrimSpace(text[s.From:s.To]) == "" {
			continue
		}

		out[text[s.From:s.To]]++
	}

	return out
}

// shared is how many bytes of token two lines carry in common, unordered: an
// upper bound on what an ordered match can find, so pairs that could never
// reach the floor are not asked to.
func shared(a, b map[string]int) int {
	if len(a) > len(b) {
		a, b = b, a
	}

	total := 0
	for token, n := range a {
		total += min(n, b[token]) * len(token)
	}

	return total
}

func refOf(path string, l Line) LineRef {
	return LineRef{Path: path, Kind: l.Kind, Old: l.Old, New: l.New}
}

// tokenCap bounds the pairing, which is quadratic in the tokens of a line. Past
// it the line is a minified bundle or a data blob, where a word-level mark says
// nothing a reader can use anyway.
const tokenCap = 400

// sameEnough is how much of the two lines has to be shared before a word-level
// mark is drawn instead of the whole line. It is measured against their mean
// length rather than the longer, so a line that gained a clause is still marked
// while two lines that merely share their indent are not.
const sameEnough = 0.4

// mark reports the runs of each line the other does not carry, and false where
// the two have too little in common to pair.
func mark(before, after string) ([]Span, []Span, bool) {
	if before == after {
		return nil, nil, false
	}

	left, right := tokens(before), tokens(after)
	if len(left) > tokenCap || len(right) > tokenCap {
		return nil, nil, false
	}

	keptLeft, keptRight := commonSpans(before, left, after, right)

	shared := 0
	for _, at := range keptLeft {
		shared += left[at].To - left[at].From
	}

	const sides = 2

	if mean := float64(len(before)+len(after)) / sides; mean == 0 ||
		float64(shared)/mean < sameEnough {
		return nil, nil, false
	}

	gone, came := gaps(left, keptLeft), gaps(right, keptRight)
	trim(gone, came, before, after)

	return gone, came, true
}

// trim narrows each pair of gap runs to the bytes that actually differ, so a
// rename's unchanged affix — `_x` becoming `x`, a prefix swapped — marks its
// difference rather than the whole token. Runs pair positionally, so an uneven
// count is left alone rather than trimmed blind.
func trim(before, after []Span, left, right string) {
	if len(before) != len(after) {
		return
	}

	for i := range before {
		b, a := &before[i], &after[i]
		bl, ar := left, right

		for b.From < b.To && a.From < a.To &&
			bl[b.From] < 0x80 && bl[b.From] == ar[a.From] {
			b.From++
			a.From++
		}

		for b.From < b.To && a.From < a.To &&
			bl[b.To-1] < 0x80 && bl[b.To-1] == ar[a.To-1] {
			b.To--
			a.To--
		}
	}
}

// tokens splits a line into the units an edit is measured in: a run of word
// bytes, a run of spaces, or one byte of anything else. Splitting on whitespace
// alone marks `f(a, b)` as wholly changed when only `b` moved.
func tokens(s string) []Span {
	var out []Span

	for at := 0; at < len(s); {
		end := at + 1
		for end < len(s) && sameClass(s[at], s[end]) {
			end++
		}

		out = append(out, Span{From: at, To: end})
		at = end
	}

	return out
}

func sameClass(a, b byte) bool {
	if wordByte(a) && wordByte(b) {
		return true
	}

	return a == ' ' && b == ' '
}

// wordByte reports whether a byte joins a word run. Every byte of a multi-byte
// rune is over 0x7f and joins one, which keeps an identifier in a non-Latin
// script a single token rather than one per byte.
func wordByte(c byte) bool {
	return c >= 0x80 || c == '_' ||
		(c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// common is the longest subsequence of tokens both lines carry in order,
// reported as the index of each match on either side.
func common(left, right []string) ([]int, []int) {
	return commonWhere(len(left), len(right), func(i, j int) bool { return left[i] == right[j] })
}

func commonWhere(n, m int, eq func(i, j int) bool) ([]int, []int) {
	// table[i*cols+j] is the length of the longest common subsequence of the
	// suffixes starting at left[i] and right[j]. It is one flat slice rather
	// than a row per i, because a line's pairing calls this once per block and
	// the row allocations were the pairing's own tax.
	cols := m + 1
	table := make([]int, (n+1)*cols)
	at := func(i, j int) int { return table[i*cols+j] }
	set := func(i, j, v int) { table[i*cols+j] = v }

	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if eq(i, j) {
				set(i, j, at(i+1, j+1)+1)

				continue
			}

			set(i, j, max(at(i+1, j), at(i, j+1)))
		}
	}

	var atLeft, atRight []int

	for i, j := 0, 0; i < n && j < m; {
		switch {
		case eq(i, j):
			atLeft = append(atLeft, i)
			atRight = append(atRight, j)
			i++
			j++
		case at(i+1, j) >= at(i, j+1):
			i++
		default:
			j++
		}
	}

	return atLeft, atRight
}

// commonSpans is common for tokens carried as spans of two source strings,
// which compares the text where it lies rather than cutting string slices of
// it first.
func commonSpans(lsrc string, left []Span, rsrc string, right []Span) ([]int, []int) {
	return commonWhere(len(left), len(right), func(i, j int) bool {
		l, r := left[i], right[j]

		return l.To-l.From == r.To-r.From && lsrc[l.From:l.To] == rsrc[r.From:r.To]
	})
}

// gaps is every token not in kept, merged into runs, which is what a reader
// sees marked.
func gaps(all []Span, kept []int) []Span {
	held := make([]bool, len(all))
	for _, at := range kept {
		held[at] = true
	}

	var out []Span

	for i, s := range all {
		if held[i] {
			continue
		}

		if n := len(out); n > 0 && out[n-1].To == s.From {
			out[n-1].To = s.To

			continue
		}

		out = append(out, s)
	}

	return out
}
