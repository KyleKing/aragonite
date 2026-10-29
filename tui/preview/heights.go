package preview

// ruleHeight is the row a section's labeled rule takes before any rows fit.
const ruleHeight = 1

// floor is the fewest lines a section is drawn in: its rule, plus its first
// row when it has one. An empty section collapses to the rule alone.
func floor(s *Section) int {
	return ruleHeight + min(len(s.Rows), 1)
}

// want is how many lines beyond its floor a section would use given room.
func want(s *Section) int {
	return len(s.Rows) - min(len(s.Rows), 1)
}

// room is how many more lines a section would still take at its height.
func room(s *Section, height int) int {
	return want(s) - (height - floor(s))
}

// Heights shares available lines between the sections. Every section keeps its
// floor so none is ever fully hidden, every section that wants more gets up to
// an equal share before relevance is consulted so a low-scoring list is never
// starved beside a high-scoring one, the focused section is served next so its
// selection stays visible, and what remains is split in proportion to
// relevance.
func Heights(sections []Section, focused, available int) []int {
	heights := make([]int, len(sections))
	if len(sections) == 0 {
		return heights
	}

	floors := 0
	for i := range sections {
		heights[i] = floor(&sections[i])
		floors += heights[i]
	}

	surplus := available - floors
	if surplus <= 0 {
		return heights
	}

	surplus = grantFairShares(heights, sections, available, surplus)
	surplus = grantWant(heights, sections, focused, surplus)
	shareByRelevance(heights, sections, surplus)

	return heights
}

// grantFairShares gives every section that wants them the lines an even split
// would allow, before relevance is consulted at all. This is what keeps
// relevance from being a winner-take-all score.
func grantFairShares(heights []int, sections []Section, available, surplus int) int {
	for i := range sections {
		fair := max(available/len(sections)-floor(&sections[i]), 0)
		take := min(want(&sections[i]), fair, surplus)
		heights[i] += take
		surplus -= take
	}

	return surplus
}

// grantWant tops one section up to everything it would show, used for the
// focused section so its selection stays visible.
func grantWant(heights []int, sections []Section, i, surplus int) int {
	if i < 0 || i >= len(sections) {
		return surplus
	}

	take := min(max(room(&sections[i], heights[i]), 0), surplus)
	heights[i] += take

	return surplus - take
}

// shareByRelevance splits what is left between the sections still short of
// their content, in proportion to how much each has to say.
func shareByRelevance(heights []int, sections []Section, surplus int) {
	for surplus > 0 {
		weight := 0
		for i := range sections {
			if room(&sections[i], heights[i]) > 0 {
				weight += sections[i].Relevance
			}
		}
		if weight == 0 {
			return
		}

		spent := 0
		for i := range sections {
			room := room(&sections[i], heights[i])
			if room <= 0 {
				continue
			}

			share := min(max(surplus*sections[i].Relevance/weight, 1), room, surplus-spent)
			heights[i] += share
			spent += share
		}

		if spent == 0 {
			return
		}
		surplus -= spent
	}
}
