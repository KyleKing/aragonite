package palette

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/kyleking/aragonite/fuzzy"
)

// Section names drawn as labeled rules over an unfiltered list.
const (
	SectionHere    = "here"
	SectionRecent  = "recent"
	SectionOff     = "off here"
	SectionMatches = "matches"
)

// RecentLimit is how many recently run entries an unfiltered list lifts into its recent section.
const RecentLimit = 5

// Ranking weights over a fuzzy score. Being runnable here outweighs having run lately, which
// outweighs nothing; an entry that is off here sinks but stays listed. A query that is exactly an
// entry's key ranks that entry first, so typing what a reader half-remembers pressing finds it.
const (
	hereBonus    = 40
	recentBonus  = 30
	offPenalty   = 80
	exactKey     = 400
	recentWindow = 7 * 24 * time.Hour
	// AliasShare is the part of an alias match's score kept, so the entry's own words win a tie.
	aliasShare = 9
	shareOf    = 10
)

// Hit is one entry a list shows, with where the query matched its description for highlighting,
// and the alias it was found by when that is how.
type Hit struct {
	// Via is the alias the query matched, drawn beside the entry so a git word teaches the jj one.
	Via string
	// Pos are the rune indices in What the query matched, nil when it matched elsewhere.
	Pos []int
	Entry
	Score int
}

// Section is a labeled run of hits.
type Section struct {
	Name string
	Hits []Hit
}

// Rank orders entries for query. An empty query answers sections in a fixed order: here (runnable,
// in the order given), recent (the last RecentLimit run that are runnable, newest first), and off
// here (with their reasons). A query answers one section of matches, best first: the fuzzy score
// over the entry's description, action, key, command, and aliases, plus a bonus for being
// runnable here and a decaying one for a recent run, minus a penalty for being off.
func Rank(entries []Entry, query string, now time.Time) []Section {
	if strings.TrimSpace(query) == "" {
		return sections(entries)
	}

	hits := make([]Hit, 0, len(entries))

	for i := range entries {
		if h, ok := score(&entries[i], query, now); ok {
			hits = append(hits, h)
		}
	}

	slices.SortStableFunc(hits, func(a, b Hit) int { return cmp.Compare(b.Score, a.Score) })

	return []Section{{Name: SectionMatches, Hits: hits}}
}

func sections(entries []Entry) []Section {
	var here, recent, off []Hit

	for i := range entries {
		h := Hit{Entry: entries[i]}

		switch {
		case h.Off:
			off = append(off, h)
		case !h.Recent.IsZero():
			recent = append(recent, h)
		default:
			here = append(here, h)
		}
	}

	slices.SortStableFunc(recent, func(a, b Hit) int { return b.Recent.Compare(a.Recent) })

	if len(recent) > RecentLimit {
		// The rest fall back into here, in the order they were given.
		here = append(here, recent[RecentLimit:]...)
		recent = recent[:RecentLimit]
		order := map[string]int{}

		for i := range entries {
			order[entries[i].Action] = i
		}

		slices.SortStableFunc(here, func(a, b Hit) int { return cmp.Compare(order[a.Action], order[b.Action]) })
	}

	return slices.DeleteFunc([]Section{
		{Name: SectionHere, Hits: here},
		{Name: SectionRecent, Hits: recent},
		{Name: SectionOff, Hits: off},
	}, func(s Section) bool { return len(s.Hits) == 0 })
}

// Groups is the entries filed under their legend headings, in the order the headings first
// appear, which is how browse mode lists a screen's keys before anything is typed. Off entries
// stay in place, drawn with their reasons.
func Groups(entries []Entry) []Section {
	var out []Section

	at := map[string]int{}

	for j := range entries {
		group := entries[j].Group

		i, ok := at[group]
		if !ok {
			i = len(out)
			at[group] = i
			out = append(out, Section{Name: group})
		}

		out[i].Hits = append(out[i].Hits, Hit{Entry: entries[j]})
	}

	return out
}

func score(e *Entry, query string, now time.Time) (Hit, bool) {
	q := strings.TrimSpace(query)

	h, found := match(e, q)
	if !found {
		return h, false
	}

	if q == e.Key || slices.Contains(e.Keys, q) {
		h.Score += exactKey
	}

	if e.Off {
		h.Score -= offPenalty
	} else {
		h.Score += hereBonus
	}

	if age := now.Sub(e.Recent); !e.Recent.IsZero() && age < recentWindow {
		h.Score += int(float64(recentBonus) * (1 - float64(age)/float64(recentWindow)))
	}

	return h, true
}

// match is the best fuzzy score of q over the entry's words: its description, which keeps the
// positions for highlighting, then its action, key, command, and group, then its aliases at a
// discount.
func match(e *Entry, q string) (Hit, bool) {
	h := Hit{Entry: *e}
	found := false

	if s, pos, ok := fuzzy.Score(e.What, q); ok {
		h.Score, h.Pos, found = s, pos, true
	}

	for _, text := range []string{e.Action, e.Key, e.Cmd, e.Group} {
		if s, _, ok := fuzzy.Score(text, q); ok && (!found || s > h.Score) {
			h.Score, h.Pos, found = s, nil, true
		}
	}

	for _, alias := range e.Also {
		if s, _, ok := fuzzy.Score(alias, q); ok && (!found || s*aliasShare/shareOf > h.Score) {
			h.Score, h.Pos, h.Via, found = s*aliasShare/shareOf, nil, alias, true
		}
	}

	return h, found
}
