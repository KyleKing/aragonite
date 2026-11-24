// Package fuzzy scores how well a typed query matches a line of text as an in-order subsequence,
// the way a command palette, a file finder, or a legend filter ranks what it holds. A run of
// adjacent letters, a letter starting a word, and a letter in the query's own case each score
// more, and a match late in the text scores less, so "sq" ranks "squash" over "rebase -s quiet".
//
// The scorer is jj-diff's file and revision finder, generalized: positions come back as rune
// indices for highlighting, the case fold is Unicode-aware, and the best placement of the query is
// found rather than the leftmost, so an early stray letter cannot hide a better match later in the
// text.
package fuzzy

import (
	"cmp"
	"slices"
	"unicode"
	"unicode/utf8"
)

// Scoring weights. The run bonus compounds across adjacent matches, so letters matched together
// outscore the same letters spread apart, and the position penalty keeps an early match ahead of
// a late one.
const (
	matchScore    = 100
	runStep       = 60
	caseBonus     = 10
	boundaryBonus = 50
	startBonus    = 60
	positionShare = 3
	// Gap is how far behind a match the placement before it must be for the two not to be a run.
	gap = 2
)

// Score is how well query matches text as an in-order subsequence, folding case, with the rune
// index in text of each query rune it matched, for highlighting. Its ok is false when some rune of
// the query is missing from text; an empty query matches everything with a score of zero.
// Whitespace in the query is skipped, so "rebase -b" matches "jj rebase -b <rev>".
func Score(text, query string) (int, []int, bool) {
	q := fold(query)
	if len(q) == 0 {
		return 0, nil, true
	}

	t := []rune(text)
	lower := make([]rune, len(t))

	for i, r := range t {
		lower[i] = unicode.ToLower(r)
	}

	if !subsequence(lower, q) {
		return 0, nil, false
	}

	return best(t, lower, q)
}

// qrune is one rune of the query, as typed and folded.
type qrune struct{ typed, lower rune }

func fold(query string) []qrune {
	out := make([]qrune, 0, utf8.RuneCountInString(query))

	for _, r := range query {
		if unicode.IsSpace(r) {
			continue
		}

		out = append(out, qrune{typed: r, lower: unicode.ToLower(r)})
	}

	return out
}

// subsequence is the cheap test that rules most texts out before any scoring.
func subsequence(lower []rune, q []qrune) bool {
	j := 0
	for _, r := range lower {
		if r == q[j].lower {
			j++
			if j == len(q) {
				return true
			}
		}
	}

	return false
}

// cell is the best way found to match the query up to one rune ending at one place in the text.
type cell struct {
	score, run, from int
	ok               bool
}

// best finds the highest-scoring placement of q in t: for each query rune and each place it could
// match, the better of continuing a run from the place just before or starting fresh after the
// best earlier placement. Each row keeps a running best so the search is rows times runes.
func best(t, lower []rune, q []qrune) (int, []int, bool) {
	n := len(lower)
	flat := make([]cell, n*len(q))
	rows := make([][]cell, len(q))

	for j, r := range q {
		rows[j] = flat[j*n : (j+1)*n]
		lead := cell{from: -1}

		for i := range n {
			// lead is the best placement at least two places behind i, so a match at i never
			// counts as adjacent to it.
			if behind := i - gap; j > 0 && behind >= 0 {
				if c := rows[j-1][behind]; c.ok && (!lead.ok || c.score > lead.score) {
					lead = cell{score: c.score, from: behind, ok: true}
				}
			}

			if lower[i] != r.lower {
				continue
			}

			rows[j][i] = extend(rows, t, j, i, r.typed, lead)
		}
	}

	return pick(rows, n)
}

// extend scores matching query rune j at text place i, from the best of the runs before it.
func extend(rows [][]cell, t []rune, j, i int, typed rune, lead cell) cell {
	here := matchScore + place(t, i, typed)
	if j == 0 {
		return cell{score: here, from: -1, ok: true}
	}

	out := cell{}

	if lead.ok {
		out = cell{score: lead.score + here, from: lead.from, ok: true}
	}

	if i > 0 {
		if prev := rows[j-1][i-1]; prev.ok {
			run := prev.run + runStep
			if s := prev.score + here + run; !out.ok || s > out.score {
				out = cell{score: s, run: run, from: i - 1, ok: true}
			}
		}
	}

	return out
}

// pick walks back from the best final placement to the positions that made it.
func pick(rows [][]cell, n int) (int, []int, bool) {
	last := rows[len(rows)-1]
	end := -1

	for i := range n {
		if last[i].ok && (end < 0 || last[i].score > last[end].score) {
			end = i
		}
	}

	if end < 0 {
		return 0, nil, false
	}

	pos := make([]int, len(rows))
	for j, at := len(rows)-1, end; j >= 0; j-- {
		pos[j] = at
		at = rows[j][at].from
	}

	return last[end].score, pos, true
}

// place is what matching at index i is worth beyond the match itself: the query's own case, the
// start of the text or of a word, less a share of how far in it is.
func place(t []rune, i int, typed rune) int {
	score := 0
	if t[i] == typed {
		score += caseBonus
	}

	switch {
	case i == 0:
		score += startBonus
	case boundary(t[i-1]) || (unicode.IsLower(t[i-1]) && unicode.IsUpper(t[i])):
		score += boundaryBonus
	}

	return score - i/positionShare
}

func boundary(prev rune) bool {
	switch prev {
	case '/', '_', '-', '.', ':', '(', '[', '<':
		return true
	}

	return unicode.IsSpace(prev)
}

// Match is one item Rank kept: its score and where the query matched the item's text.
type Match[T any] struct {
	Item  T
	Pos   []int
	Score int
	// Index is the item's place in the input, which breaks ties so equal scores keep the caller's
	// order.
	Index int
}

// Rank keeps the items whose text matches query, best first, ties in input order. An empty query
// keeps every item in input order.
func Rank[T any](query string, items []T, text func(T) string) []Match[T] {
	out := make([]Match[T], 0, len(items))

	for i, it := range items {
		score, pos, ok := Score(text(it), query)
		if ok {
			out = append(out, Match[T]{Item: it, Score: score, Pos: pos, Index: i})
		}
	}

	slices.SortStableFunc(out, func(a, b Match[T]) int { return cmp.Compare(b.Score, a.Score) })

	return out
}
