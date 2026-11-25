package keyhint_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/framecheck"
	"github.com/kyleking/aragonite/tui/keyhint"
)

// plain styles keep the test about the placement of the brackets rather than
// about the escapes lipgloss wraps them in.
var plain = keyhint.Styles{Key: lipgloss.NewStyle(), Text: lipgloss.NewStyle()}

func TestOne(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		hint keyhint.Hint
	}{
		{"at the start", "[p]r", keyhint.Hint{Key: "p", What: "pr"}},
		{"a word in", "post [o]ne", keyhint.Hint{Key: "o", What: "post one"}},
		{"inside a word", "s[u]bmit", keyhint.Hint{Key: "u", What: "submit"}},
		{"shifted keeps its case", "[S]ubmit", keyhint.Hint{Key: "S", What: "submit"}},
		{"not in the word", "[tab] switch", keyhint.Hint{Key: "tab", What: "switch"}},
		{"punctuation", "[/] search", keyhint.Hint{Key: "/", What: "search"}},
		{"nothing to sit in", "[q] ", keyhint.Hint{Key: "q", What: ""}},
		{"a closing bracket", "] go to", keyhint.Hint{Key: "]", What: "go to"}},
		{"an opening bracket", "[ go back", keyhint.Hint{Key: "[", What: "go back"}},
		{"a bracket in the word", "] hunk", keyhint.Hint{Key: "]", What: "] hunk"}},
		{"a bracket off", "] go to", keyhint.Hint{Key: "]", What: "go to", Off: true}},
		{"a bracketed key off", "[x] y", keyhint.Hint{Key: "x", What: "y", Off: true}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := keyhint.One(plain, tc.hint); got != tc.want {
				t.Errorf("One() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLine(t *testing.T) {
	t.Parallel()

	got := keyhint.Line(plain, []keyhint.Hint{
		{Key: "e", What: "edit"}, {Key: "tab", What: "switch"},
	})

	if want := "[e]dit" + keyhint.Gap + "[tab] switch"; got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
}

func TestHelp(t *testing.T) {
	t.Parallel()

	got := keyhint.Help(plain, []keyhint.Hint{
		{Key: "j / k", What: "move a line"},
		{Key: "S then S / a / r / c", What: "submit"},
		{What: "The keys above are the whole of it."},
	}, 60)

	want := []string{
		"                 j / k  move a line",
		"  S then S / a / r / c  submit",
		"                        The keys above are the whole of it.",
	}

	if len(got) != len(want) {
		t.Fatalf("Help() gave %d lines, want %d: %q", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A heading sits in the description column, so the keys under it keep the one
// edge a reader scans, and it is drawn in its own face.
func TestHelpDrawsAHeadingWithTheDescriptions(t *testing.T) {
	t.Parallel()

	hints := []keyhint.Hint{
		{What: "moving", Head: true},
		{Key: "j / k", What: "move a line"},
	}

	// Measured with no face on the heading, since an escape in the line would
	// make a byte index lie about the column.
	got := keyhint.Help(plain, hints, 40)
	if len(got) != 2 {
		t.Fatalf("Help() gave %d lines, want 2: %q", len(got), got)
	}

	if head, desc := strings.Index(got[0], "moving"), strings.Index(got[1], "move a line"); head != desc {
		t.Errorf("the heading starts at column %d and the description at %d: %q", head, desc, got[0])
	}

	styled := plain
	styled.Head = lipgloss.NewStyle().Bold(true)

	if got := keyhint.Help(styled, hints, 40); !strings.Contains(got[0], styled.Head.Render("moving")) {
		t.Errorf("the heading is not drawn in Head: %q", got[0])
	}
}

// A legend has to fit the frame it is drawn in, since a help screen that wraps
// is the one screen a reader cannot fall back on.
func TestHelpFitsItsWidth(t *testing.T) {
	t.Parallel()

	for _, width := range []int{40, 60, 80, 120} {
		for _, line := range keyhint.Help(plain, []keyhint.Hint{
			{Key: "S then S / a / r / c", What: strings.Repeat("long ", 40)},
			{What: strings.Repeat("prose ", 40)},
		}, width) {
			if lipgloss.Width(line) > width {
				t.Errorf("at %d columns a line is %d wide: %q", width, lipgloss.Width(line), line)
			}
		}
	}
}

func TestFit(t *testing.T) {
	t.Parallel()

	hints := []keyhint.Hint{
		{Key: "j", What: "move", Priority: 2},
		{Key: "e", What: "edit", Priority: 1},
		{Key: "d", What: "delete", Priority: 3, Off: true},
		{Key: "q", What: "quit", Priority: 9},
	}

	tests := []struct {
		name  string
		want  string
		width int
	}{
		{"all fit, Off stays drawn", "[j] move  [e]dit  [d] delete  [q]uit", 40},
		{"Off goes before a lower priority", "[j] move  [e]dit  [q]uit", 30},
		{"then the lowest priority", "[j] move  [q]uit", 20},
		{"then the next", "[q]uit", 10},
		{"the last one is clipped", "[q]u", 4},
		{"no room draws nothing", "", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := keyhint.Fit(plain, hints, tc.width)
			if got != tc.want {
				t.Errorf("Fit(%d) = %q, want %q", tc.width, got, tc.want)
			}
		})
	}
}

func TestBareDropsTheBracketsAndKeepsTheGap(t *testing.T) {
	t.Parallel()

	bare := keyhint.Styles{Key: lipgloss.NewStyle(), Text: lipgloss.NewStyle(), Bare: true, Gap: "   "}
	hints := []keyhint.Hint{
		{Key: "enter", What: "review", Priority: 9},
		{Key: "C", What: "check out", Priority: 1},
		{Key: "s", What: "sort", Off: true},
		{Key: "?", What: "help", Priority: 9},
	}

	tests := []struct {
		name  string
		want  string
		width int
	}{
		{"all fit", "enter review   C check out   s sort   ? help", 60},
		{"the off key goes first", "enter review   C check out   ? help", 36},
		{"then the lowest priority, whole", "enter review   ? help", 30},
		{"never mid-word", "enter review   ? help", 21},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := keyhint.Fit(bare, hints, tc.width); got != tc.want {
				t.Errorf("Fit(%d) = %q, want %q", tc.width, got, tc.want)
			}
		})
	}

	if got := keyhint.One(bare, keyhint.Hint{Key: "]", What: "next", Off: true}); got != "] next" {
		t.Errorf("an off bare hint = %q, want it flat and unbracketed", got)
	}
}

func TestPanel_AlignsKeysAndCountsAndFitsTheWidth(t *testing.T) {
	t.Parallel()

	s := keyhint.PanelStyles{Frame: lipgloss.NewStyle().Border(lipgloss.NormalBorder())}
	hints := []keyhint.Hint{
		{Key: "a", What: "all", Count: "12", On: true},
		{Key: "esc", What: "a description much too long", Count: "3", Kids: []keyhint.Hint{{Key: "x"}}},
	}

	tests := []struct {
		name  string
		want  []string
		width int
	}{
		{name: "roomy", width: 60, want: []string{
			"┌─────────────────────────────────────┐",
			"│  a  all                           12│",
			"│esc  a description much too long…   3│",
			"└─────────────────────────────────────┘",
		}},
		{name: "tight", width: 20, want: nil},
		{name: "no room inside the frame", width: 2, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := keyhint.Panel(s, hints, tc.width)
			framecheck.Require(t, tc.name, strings.Join(got, "\n"), framecheck.MaxWidth(max(tc.width, 0)))
			if tc.want == nil {
				for _, l := range got {
					if w := lipgloss.Width(l); w > tc.width {
						t.Errorf("row %q is %d cells wide on a %d-cell width", l, w, tc.width)
					}
				}

				return
			}

			if !slices.Equal(got, tc.want) {
				t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestFreshDrawsKeyApart(t *testing.T) {
	t.Parallel()

	s := keyhint.Styles{Key: lipgloss.NewStyle()}
	seen := map[string]int{}

	for level := range 4 {
		got := keyhint.One(s, keyhint.Hint{Key: "F", What: "fixme", Fresh: level})
		if ansi.Strip(got) != "[F]ixme" {
			t.Fatalf("level %d changed the text: %q", level, got)
		}

		if prev, dup := seen[got]; dup {
			t.Fatalf("levels %d and %d draw alike: %q", prev, level, got)
		}

		seen[got] = level
	}
}

func TestWhy_DrawnAfterAnOffRow(t *testing.T) {
	t.Parallel()

	h := keyhint.Hint{Key: "x", What: "resolve", Off: true, Why: "@ has no conflict"}

	for name, lines := range map[string][]string{
		"Help":   keyhint.Help(plain, []keyhint.Hint{h}, 60),
		"Column": keyhint.Column(plain, []keyhint.Hint{h}, 60),
		"Panel":  keyhint.Panel(keyhint.PanelStyles{Styles: plain}, []keyhint.Hint{h}, 60),
	} {
		if !strings.Contains(strings.Join(lines, "\n"), "resolve · @ has no conflict") {
			t.Errorf("%s = %q, want the reason after the description", name, lines)
		}
	}

	on := h
	on.Off = false

	if got := strings.Join(keyhint.Help(plain, []keyhint.Hint{on}, 60), ""); strings.Contains(got, "conflict") {
		t.Errorf("Help draws Why on a row that is not Off: %q", got)
	}
}

func TestSuggest_UsesSuggestFaceOnlyWhenSet(t *testing.T) {
	t.Parallel()

	s := plain
	s.Bare = true
	s.Suggest = lipgloss.NewStyle().Underline(true)

	h := keyhint.Hint{Key: "d", What: "describe", Suggest: true}

	if got := keyhint.One(s, h); strings.Contains(got, "\x1b[4") {
		t.Errorf("One drew the Suggest face without Suggesting: %q", got)
	}

	s.Suggesting = true
	if got := keyhint.One(s, h); got == keyhint.One(plain, keyhint.Hint{Key: "d", What: "describe"}) {
		t.Errorf("One ignored Suggest: %q", got)
	}
}

// footerHints is a footer's worth of hints, more than 80 cells hold.
func footerHints() []keyhint.Hint {
	return []keyhint.Hint{
		{Key: "?", What: "help", Pin: true},
		{Key: ":", What: "commands", Pin: true},
		{Key: "d", What: "describe", Priority: 5},
		{Key: "n", What: "new", Priority: 5},
		{Key: "s", What: "squash", Priority: 4},
		{Key: "S", What: "split", Priority: 3},
		{Key: "r", What: "rebase", Priority: 4},
		{Key: "a", What: "abandon", Priority: 2},
		{Key: "e", What: "edit", Priority: 3},
		{Key: "b", What: "bookmark", Priority: 2},
		{Key: "u", What: "undo", Priority: 1},
		{Key: "p", What: "push", Priority: 1, Off: true},
		{Key: "f", What: "fetch", Priority: 1},
		{Key: "/", What: "search"},
	}
}

func colored() keyhint.Styles {
	return keyhint.Styles{
		Key:  lipgloss.NewStyle().Foreground(lipgloss.Color("#8aadf4")).Bold(true),
		Text: lipgloss.NewStyle().Foreground(lipgloss.Color("#a5adcb")),
		Off:  lipgloss.NewStyle().Foreground(lipgloss.Color("#6e738d")),
	}
}

// BenchmarkFit is a footer shed to 80 cells, which a tool draws per frame.
func BenchmarkFit(b *testing.B) {
	s, hints := colored(), footerHints()

	b.ReportAllocs()

	for b.Loop() {
		_ = keyhint.Fit(s, hints, 80)
	}
}

func TestFit_MatchesSheddingOneAtATime(t *testing.T) {
	t.Parallel()

	s, hints := colored(), footerHints()
	for width := range 140 {
		got := keyhint.Fit(s, hints, width)

		kept := slices.Clone(hints)
		want := ""
		if width > 0 {
			want = ansi.Truncate(slowFit(s, kept, width), width, "")
		}

		if got != want {
			t.Fatalf("width %d:\n got %q\nwant %q", width, ansi.Strip(got), ansi.Strip(want))
		}
	}
}

// slowFit is the definition Fit keeps: render the whole line, shed the hint
// Fit would shed, and measure again.
func slowFit(s keyhint.Styles, hints []keyhint.Hint, width int) string {
	for {
		line := keyhint.Line(s, hints)
		if len(hints) <= 1 || lipgloss.Width(line) <= width {
			return line
		}

		hints = slices.Delete(hints, victim(hints), victim(hints)+1)
	}
}

// victim restates Fit's shedding order: off before on, then the lowest
// priority, then the later of two equals, and a pinned hint last of all.
func victim(hints []keyhint.Hint) int {
	v := 0

	for i, h := range hints {
		cur := hints[v]
		if cur.Pin != h.Pin {
			if cur.Pin {
				v = i
			}

			continue
		}

		if (h.Off && !cur.Off) || (h.Off == cur.Off && h.Priority <= cur.Priority) {
			v = i
		}
	}

	return v
}

func TestMemo_DrawsOncePerKey(t *testing.T) {
	t.Parallel()

	var (
		m     keyhint.Memo[[2]int]
		draws int
	)

	draw := func() string { draws++; return "line" }

	for _, k := range [][2]int{{1, 80}, {1, 80}, {2, 80}, {2, 80}, {1, 80}} {
		if got := m.Line(k, draw); got != "line" {
			t.Fatalf("Line = %q", got)
		}
	}

	if draws != 3 {
		t.Errorf("drew %d times, want 3 (one per change of key)", draws)
	}

	m.Reset()
	m.Line([2]int{1, 80}, draw)

	if draws != 4 {
		t.Errorf("Reset did not empty the memo: %d draws", draws)
	}
}

// BenchmarkMemo is a footer redrawn with its key unchanged.
func BenchmarkMemo(b *testing.B) {
	s, hints := colored(), footerHints()

	var m keyhint.Memo[int]

	b.ReportAllocs()

	for b.Loop() {
		_ = m.Line(1, func() string { return keyhint.Fit(s, hints, 80) })
	}
}

// holes reports each unstyled cell that sits between two styled ones in s, as
// a terminal would draw it: a hint drawn on a band shows such a cell as a gap
// in the band.
func holes(s string) []int {
	var (
		styled  bool
		cells   []bool
		inSeq   bool
		seq     strings.Builder
		escaped bool
	)

	for _, r := range s {
		switch {
		case escaped:
			escaped = false
			inSeq = r == '['

			seq.Reset()
		case inSeq:
			if r < 0x40 || r > 0x7e {
				seq.WriteRune(r)

				continue
			}

			inSeq = false

			if r == 'm' {
				p := seq.String()
				styled = p != "" && p != "0"
			}
		case r == '\x1b':
			escaped = true
		default:
			cells = append(cells, styled)
		}
	}

	first, last := slices.Index(cells, true), -1
	for i, on := range cells {
		if on {
			last = i
		}
	}

	var out []int

	for i := first + 1; i < last && first >= 0; i++ {
		if !cells[i] {
			out = append(out, i)
		}
	}

	return out
}

// A hint drawn on a band (a background in its faces, or reverse video where
// the profile has no color) is one run: the cell between its key and its word
// carries the band like the rest, in every profile a terminal may downsample
// it to.
func TestHint_NoHoleBetweenKeyAndWordOnABand(t *testing.T) {
	t.Parallel()

	band := lipgloss.Color("#363a4f")
	colored := keyhint.Styles{
		Key:  lipgloss.NewStyle().Background(band).Foreground(lipgloss.Color("#8aadf4")).Bold(true),
		Text: lipgloss.NewStyle().Background(band).Foreground(lipgloss.Color("#cad3f5")),
	}
	reversed := keyhint.Styles{
		Key:  lipgloss.NewStyle().Reverse(true).Bold(true),
		Text: lipgloss.NewStyle().Reverse(true),
	}

	hints := []keyhint.Hint{
		{Key: "enter", What: "review"},
		{Key: "C", What: "check out", Fresh: 3},
		{Key: "tab", What: "switch"},
		{Key: "x", What: "go"},
	}

	profiles := []struct {
		name    string
		styles  keyhint.Styles
		profile colorprofile.Profile
	}{
		{"truecolor", colored, colorprofile.TrueColor},
		{"256 colors", colored, colorprofile.ANSI256},
		{"16 colors", colored, colorprofile.ANSI},
		{"16 colors reversed", reversed, colorprofile.ANSI},
		{"NO_COLOR reversed", reversed, colorprofile.Ascii},
	}

	for _, p := range profiles {
		for _, bare := range []bool{true, false} {
			for _, h := range hints {
				t.Run(fmt.Sprintf("%s/bare=%t/%s", p.name, bare, h.Key), func(t *testing.T) {
					t.Parallel()

					s := p.styles
					s.Bare = bare

					var b bytes.Buffer

					w := &colorprofile.Writer{Forward: &b, Profile: p.profile}
					if _, err := w.WriteString(keyhint.One(s, h)); err != nil {
						t.Fatal(err)
					}

					if got := holes(b.String()); len(got) > 0 {
						t.Errorf("cells %v of %q are unstyled between styled runs: %q",
							got, ansi.Strip(b.String()), b.String())
					}
				})
			}
		}
	}
}

func TestBare_AKeyWithNoWordIsTheKeyAlone(t *testing.T) {
	t.Parallel()

	s := plain
	s.Bare = true

	for _, h := range []keyhint.Hint{{Key: "q"}, {Key: "q", Off: true}} {
		if got := keyhint.One(s, h); got != "q" {
			t.Errorf("One(%+v) = %q, want the key alone", h, got)
		}
	}
}
