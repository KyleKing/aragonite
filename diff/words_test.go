package diff_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/diff"
)

// marked renders a line with its changed runs bracketed, which is what the
// screen draws as a band and what a test can read.
func marked(text string, spans []diff.Span) string {
	var (
		b  strings.Builder
		at int
	)

	for _, s := range spans {
		b.WriteString(text[at:s.From] + "[" + text[s.From:s.To] + "]")
		at = s.To
	}

	return b.String() + text[at:]
}

// hunk is a one-file patch whose lines are written with their own prefix.
func hunk(lines ...string) string {
	return "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,9 +1,9 @@\n" +
		strings.Join(lines, "\n") + "\n"
}

func patchOf(before, after string) string {
	return hunk("-"+before, "+"+after)
}

// pairingOf renders every changed line of d the pairing touched, in patch
// order: its marked text, an R where it was re-indented, and the line it
// paired with.
func pairingOf(d *diff.Diff) []string {
	marks, reindented, _, mates := d.Paired()

	var out []string

	for _, l := range d.Files[0].Lines {
		ref := diff.LineRef{Path: "a.go", Kind: l.Kind, Old: l.Old, New: l.New}
		mate, paired := mates[ref]

		if !paired && !reindented[ref] {
			if _, ok := marks[ref]; !ok {
				continue
			}
		}

		row := fmt.Sprintf("%c%q", l.Kind, marked(l.Text, marks[ref]))
		if reindented[ref] {
			row += " R"
		}

		if paired {
			row += fmt.Sprintf(" ~ %c%d/%d", mate.Kind, mate.Old, mate.New)
		}

		out = append(out, row)
	}

	return out
}

type pairingCase struct {
	name  string
	patch string
	want  []string
}

func pairingCases() []pairingCase {
	return []pairingCase{
		{
			name:  "one identifier",
			patch: patchOf("\tif err := run(ctx); err != nil {", "\tif err := walk(ctx); err != nil {"),
			want: []string{
				`-"\tif err := [run](ctx); err != nil {" ~ +0/1`,
				`+"\tif err := [walk](ctx); err != nil {" ~ -1/0`,
			},
		},
		{
			name:  "an argument added",
			patch: patchOf(`return fmt.Errorf("reading: %w", err)`, `return fmt.Errorf("reading %s: %w", path, err)`),
			want: []string{
				`-"return fmt.Errorf(\"reading: %w\", err)" ~ +0/1`,
				`+"return fmt.Errorf(\"reading[ %s]: %w\", [path, ]err)" ~ -1/0`,
			},
		},
		{
			name:  "trailing text only",
			patch: patchOf("const timeout = 5", "const timeout = 30"),
			want: []string{
				`-"const timeout = [5]" ~ +0/1`,
				`+"const timeout = [30]" ~ -1/0`,
			},
		},
		{
			name:  "a rewrite is left whole",
			patch: patchOf("\tcount := len(items)", "\treturn fmt.Sprintf(\"%s/%s\", owner, repo)"),
			want:  nil,
		},
		{
			name:  "identical text pairs nothing",
			patch: hunk("-same line", "+same line"),
			want:  nil,
		},
		{
			name:  "a context line ends the block",
			patch: hunk("-const a = 1", "+const a = 2", " context", "-const b = 3", "+const b = 4"),
			want: []string{
				`-"const a = [1]" ~ +0/1`,
				`+"const a = [2]" ~ -1/0`,
				`-"const b = [3]" ~ +0/3`,
				`+"const b = [4]" ~ -3/0`,
			},
		},
		{
			name:  "an addition before a removal ends the block",
			patch: hunk("-const a = 1", "+const a = 2", "-const a = 3"),
			want: []string{
				`-"const a = [1]" ~ +0/1`,
				`+"const a = [2]" ~ -1/0`,
			},
		},
		{
			// The second removal shares fewer bytes with the addition and is
			// floored out, so the first is the only candidate however the
			// shorter line's ratio rounds.
			name: "the closer of two removals pairs",
			patch: hunk(
				"-aaaa bbbb qqqqqqqqqqqqqqqqqqqq",
				"-aaaa xxxxxxx",
				"+aaaa bbbb",
			),
			want: []string{
				`-"aaaa bbbb[ qqqqqqqqqqqqqqqqqqqq]" ~ +0/1`,
				`+"aaaa bbbb" ~ -1/0`,
			},
		},
		{
			name:  "a line the indent pairing took is not paired again",
			patch: hunk("-\tfoo(a, b)", "+\tif ok {", "+\t\tfoo(a, b)", "+\tfoo(a, c)"),
			want: []string{
				`-"\tfoo(a, b)" R ~ +0/2`,
				`+"\t[\t]foo(a, b)" R ~ -1/0`,
			},
		},
	}
}

// indentCases pair lines that say the same thing at another indent.
func indentCases() []pairingCase {
	return []pairingCase{
		{
			name: "a line pushed under a new block pairs with itself",
			patch: hunk(
				"-\tfoo(a)", "-\tbar(b)",
				"+\tif ok {", "+\t\tfoo(a)", "+\t\tbar(b)", "+\t}",
			),
			want: []string{
				`-"\tfoo(a)" R ~ +0/2`,
				`-"\tbar(b)" R ~ +0/3`,
				`+"\t[\t]foo(a)" R ~ -1/0`,
				`+"\t[\t]bar(b)" R ~ -2/0`,
			},
		},
		{
			name: "what the indent pairing leaves still pairs by match",
			patch: hunk(
				"-\tfoo(a)", "-\tconst x = 1",
				"+\tif ok {", "+\t\tfoo(a)", "+\tconst x = 2",
			),
			want: []string{
				`-"\tfoo(a)" R ~ +0/2`,
				`-"\tconst x = [1]" ~ +0/3`,
				`+"\t[\t]foo(a)" R ~ -1/0`,
				`+"\tconst x = [2]" ~ -2/0`,
			},
		},
		{
			name:  "four characters are enough to pair across an indent",
			patch: hunk("-\tx++;", "+\tif ok {", "+\t\tx++;", "+\t}"),
			want: []string{
				`-"\tx++;" R ~ +0/2`,
				`+"\t[\t]x++;" R ~ -1/0`,
			},
		},
		{
			name:  "three characters are not",
			patch: hunk("-\tx++", "+\tif ok {", "+\t\tx++", "+\t}"),
			want: []string{
				`-"\tx++" ~ +0/2`,
				`+"\t[\t]x++" ~ -1/0`,
			},
		},
		{
			name:  "a line kept as is beside an addition is not marked",
			patch: hunk("-\tkeep(it)", "+\tkeep(it)", "+\tmore()"),
			want:  nil,
		},
	}
}

func TestPaired_MarksWhatChangedAndNamesTheMate(t *testing.T) {
	t.Parallel()

	tests := append(pairingCases(), indentCases()...)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := pairingOf(diff.Parse([]byte(tc.patch))); !slices.Equal(got, tc.want) {
				t.Errorf("pairing =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(tc.want, "\n  "))
			}
		})
	}
}

// Refine, Reindented, and Mates each answer one part of Paired.
func TestPaired_AgreesWithItsParts(t *testing.T) {
	t.Parallel()

	d := diff.Parse([]byte(hunk(
		"-\tfoo(a)", "-\tconst x = 1",
		"+\tif ok {", "+\t\tfoo(a)", "+\tconst x = 2",
	)))
	marks, reindented, moved, mates := d.Paired()

	if !maps.EqualFunc(d.Refine(), marks, slices.Equal) {
		t.Errorf("Refine = %v, Paired marks = %v", d.Refine(), marks)
	}

	if !maps.Equal(d.Reindented(), reindented) || len(reindented) != 2 {
		t.Errorf("Reindented = %v, Paired = %v", d.Reindented(), reindented)
	}

	if !maps.Equal(d.Mates(), mates) || len(mates) != 4 {
		t.Errorf("Mates = %v, Paired = %v", d.Mates(), mates)
	}

	if !maps.Equal(d.Moved(), moved) || len(moved) != 0 {
		t.Errorf("Moved = %v, Paired = %v", d.Moved(), moved)
	}
}
