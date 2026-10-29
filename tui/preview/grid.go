package preview

import (
	"strings"
)

// Grid is the geometry of the sections column and its detail pane. The two
// sit side by side behind a padded vertical rule until the width is too
// narrow, where the pane stacks under the column behind a titled rule.
type Grid struct {
	// SidePercent is the column's share of the width when split.
	SidePercent int
	// SideMin is the fewest cells the column takes.
	SideMin int
	// DetailMin is the fewest cells the pane splits beside; below it the two
	// stack.
	DetailMin int
	// StackMin is the fewest rows the pane keeps when stacked, its rule
	// included.
	StackMin int
	// RulePad is the cells of space each side of the vertical rule, which is
	// the separation the two halves would otherwise draw without.
	RulePad int
	// SectionGap is the blank rows between stacked sections.
	SectionGap int
}

// Content is what a Render call draws: the sections, where the keyboard is in
// them, and the pane's item.
type Content struct {
	Sections []Section
	Detail   Detail
	// Focused is the section the cursor is in, or -1 for a display-only column.
	Focused int
	// Cursor is the row within the focused section.
	Cursor int
}

// Stacked reports whether the grid drops its side-by-side split, either
// because the caller's compact breakpoint says so or because the detail pane
// would be too narrow to read beside the column.
func (g Grid) Stacked(compact bool, width int) bool {
	return compact || width-g.sideWidth(width)-ruleWidth-ruleSides*g.RulePad < g.DetailMin
}

// sideWidth is the column's share of a split grid.
func (g Grid) sideWidth(width int) int {
	return max(width*g.SidePercent/percent, g.SideMin)
}

// DetailSize is the pane's interior at the size the caller is drawing, which
// is the geometry a scroll offset clamps against.
func (g Grid) DetailSize(compact bool, width, height int, c Content) (int, int) {
	if g.Stacked(compact, width) {
		return width, max(height-g.stackedBound(c.Sections, height)-ruleHeight, 1)
	}

	return width - g.sideWidth(width) - ruleWidth - ruleSides*g.RulePad, max(height-1, 1)
}

// stackedBound is how many rows the column takes when the pane stacks under
// it: what the sections need, less whatever the pane's guaranteed rows
// require.
func (g Grid) stackedBound(sections []Section, height int) int {
	need := g.SectionGap * max(len(sections)-1, 0)
	for i := range sections {
		need += floor(&sections[i]) + want(&sections[i])
	}

	return min(need, max(height-g.StackMin, 1))
}

// Render draws the column and the pane into exactly height rows of width
// cells: side by side behind a padded rule, or stacked behind a titled one.
func (g Grid) Render(st Styles, compact bool, width, height int, c Content) []string {
	if width <= 0 || height <= 0 {
		return nil
	}

	if g.Stacked(compact, width) {
		return g.stack(st, width, height, c)
	}

	return g.split(st, width, height, c)
}

func (g Grid) split(st Styles, width, height int, c Content) []string {
	side := g.sideWidth(width)
	dw := width - side - ruleWidth - ruleSides*g.RulePad

	left := Pad(Column(st, c.Sections, c.Focused, c.Cursor, height, side, g.SectionGap), height, side)
	right := Pad(append([]string{c.Detail.Head(st, dw, height-1)}, c.Detail.Window(height-1)...), height, dw)

	rule := st.Rule
	if c.Detail.Focused {
		rule = st.Accent
	}
	bar := strings.Repeat(" ", g.RulePad) + rule.Render(ruleGlyph) + strings.Repeat(" ", g.RulePad)

	out := make([]string, height)
	for i := range out {
		out[i] = left[i] + bar + right[i]
	}

	return out
}

func (g Grid) stack(st Styles, width, height int, c Content) []string {
	bound := g.stackedBound(c.Sections, height)
	lines := Column(st, c.Sections, c.Focused, c.Cursor, bound, width, g.SectionGap)

	pane := max(height-len(lines)-ruleHeight, 0)
	lines = append(lines, c.Detail.Rule(st, width))
	lines = append(lines, c.Detail.Window(pane)...)

	return Pad(lines, height, width)
}
