package framecheck_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/tui/framecheck"
)

type checkCase struct {
	name  string
	frame string
	opts  []framecheck.Option
	want  []string
}

func checkCases() []checkCase {
	return []checkCase{
		{
			name:  "a closed box",
			frame: "╭─ t ─╮\n│ abc │\n╰─────╯",
			opts:  []framecheck.Option{framecheck.Width(7)},
		},
		{
			name:  "escapes are not cells",
			frame: "\x1b[31m╭──╮\x1b[0m\n│\x1b[1mab\x1b[0m│\n╰──╯",
			opts:  []framecheck.Option{framecheck.Width(4)},
		},
		{
			name:  "a short row in a whole screen",
			frame: "abcd\nab\nabcd",
			opts:  []framecheck.Option{framecheck.Width(4)},
			want:  []string{"2:3 width"},
		},
		{
			name:  "a wide glyph is two cells",
			frame: "\u65e5\u672c\nabcd",
			opts:  []framecheck.Option{framecheck.Width(4)},
		},
		{
			name:  "a ragged block under its limit",
			frame: "abc\na\nabcd",
			opts:  []framecheck.Option{framecheck.MaxWidth(4)},
		},
		{
			name:  "a row over the limit",
			frame: "abcde",
			opts:  []framecheck.Option{framecheck.MaxWidth(4)},
			want:  []string{"1:5 width"},
		},
		{
			name:  "a side one column in from its corner",
			frame: "╭───╮\n│   │\n │  │\n╰───╯",
			want:  []string{"4:1 corner", "3:1 edge"},
		},
		{
			name:  "a missing right side",
			frame: "╭───╮\n│   \n╰───╯",
			want:  []string{"1:5 corner", "3:5 corner"},
		},
		{
			name:  "a title abutting the corner",
			frame: "╭─ title╮\n│       │\n╰───────╯",
			want:  []string{"1:9 corner"},
		},
		{
			name:  "a box covered by another box",
			frame: "╭─────╮\n│  ╭──┴─╮\n│  │    │\n╰──┤    │\n   ╰────╯",
		},
		{
			name:  "a box clipped by the frame's last row",
			frame: "╭───╮\n│   │",
		},
		{
			name:  "rules and scroll bars end where they like",
			frame: "── files ─────\n  ┃ a │ b\n  ┃",
		},
		{
			name:  "a text-default emoji",
			frame: "⚠ careful ▸ ok",
			want:  []string{"1:1 glyph"},
		},
		{
			name:  "a VS16 emoji is two cells or one depending on the terminal",
			frame: "\u2764\ufe0f ok",
			want:  []string{"1:1 glyph"},
		},
		{
			name:  "a wide emoji is two cells everywhere",
			frame: "\U0001F680 ok",
		},
		{
			name:  "a Nerd icon outside the Nerd set",
			frame: "\ue627 main.go",
			want:  []string{"1:1 glyph"},
		},
		{
			name:  "a Nerd icon in the Nerd set",
			frame: "\ue627 main.go",
			opts:  []framecheck.Option{framecheck.Nerd()},
		},
		{
			name:  "an allowed glyph",
			frame: "▶ play",
			opts:  []framecheck.Option{framecheck.Allow("▶")},
		},
		{
			name:  "a tee joins its rule",
			frame: "╭──┬──╮\n│  │  │\n├──┼──┤\n╰──┴──╯",
		},
		{
			name:  "a tee with a loose arm",
			frame: "╭─────╮\n│  ┬  │\n╰─────╯",
			want:  []string{"2:4 corner"},
		},
	}
}

func TestCheck(t *testing.T) {
	t.Parallel()

	for _, tc := range checkCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			faults := framecheck.Check(tc.frame, tc.opts...)
			got := make([]string, 0, len(faults))
			for _, f := range faults {
				got = append(got, fmt.Sprintf("%d:%d %s", f.Row+1, f.Col+1, f.Rule))
			}

			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("Check() = %v, want %v\n%s", got, tc.want, framecheck.Ruled(tc.frame))
			}
		})
	}
}

type recorder struct {
	msgs []string
}

func (*recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.msgs = append(r.msgs, fmt.Sprintf(format, args...))
}

func TestRequire(t *testing.T) {
	t.Parallel()

	var clean, broken recorder

	framecheck.Require(&clean, "clean", "╭╮\n╰╯")
	framecheck.Require(&broken, "broken", "╭╮\n╰ ")

	if len(clean.msgs) != 0 {
		t.Errorf("Require(clean) reported %v", clean.msgs)
	}

	if len(broken.msgs) != 1 || !strings.Contains(broken.msgs[0], "broken: 2 alignment fault(s)") ||
		!strings.Contains(broken.msgs[0], "   1 ╭╮") {
		t.Errorf("Require(broken) = %q, want the name, the count, and the ruled frame", broken.msgs)
	}
}
