package markdown_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/markdown"
)

// dependabotBody is the shape that motivated this package: a changelog dump
// where all but the first three lines hide behind <details>.
const dependabotBody = "Bumps the go-dependencies group with 2 updates in the / directory: " +
	"[github.com/alecthomas/chroma/v2](https://github.com/alecthomas/chroma) and " +
	"[github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea).\n" +
	"\n" +
	"Updates `github.com/alecthomas/chroma/v2` from 2.23.0 to 2.27.0\n" +
	"<details>\n" +
	"<summary>Release notes</summary>\n" +
	"<p><em>Sourced from <a href=\"https://github.com/alecthomas/chroma/releases\">chroma's releases</a>.</em></p>\n" +
	"<blockquote>\n" +
	"<h2>v2.27.0</h2>\n" +
	"<ul>\n" +
	"<li>a6d00fe fix(html): make mode class output opt-in</li>\n" +
	"<li>f52d015 chore: some house-keeping</li>\n" +
	"</ul>\n" +
	"</blockquote>\n" +
	"</details>\n"

// unstyled keeps these assertions about width and text rather than color.
var unstyled = markdown.Styles{}

func render(t *testing.T, body string, width, maxLines int) string {
	t.Helper()

	lines := markdown.Render(body, width, maxLines, unstyled)
	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("line %d is %d cells wide, past the %d it was given: %q", i, got, width, line)
		}
	}

	return plain(strings.Join(lines, "\n"))
}

// plain strips the ANSI styling so assertions read against the text.
func plain(s string) string {
	var b strings.Builder

	esc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			esc = true
		case esc && r == 'm':
			esc = false
		case !esc:
			b.WriteRune(r)
		}
	}

	return b.String()
}

func TestRender_DrawsEachBlockAsText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		body  string
		want  string
		width int
	}{
		{
			name:  "a release note dump folds behind its summary",
			body:  dependabotBody,
			width: 60,
			want: "Bumps the go-dependencies group with 2 updates in the /\n" +
				"directory: github.com/alecthomas/chroma/v2 and\n" +
				"github.com/charmbracelet/bubbletea.\n\n" +
				"Updates github.com/alecthomas/chroma/v2 from 2.23.0 to\n2.27.0\n" +
				"▸ Release notes (4 lines hidden)",
		},
		{name: "headings drop their hashes", body: "## What changed\ntext", want: "What changed\ntext"},
		{
			name: "list items take a bullet and keep their nesting",
			body: "- top\n    - nested",
			want: "• top\n    • nested",
		},
		{name: "task boxes survive as boxes", body: "- [x] done\n- [ ] todo", want: "☑ done\n☐ todo"},
		{
			name: "links keep the label and drop the target",
			body: "see [the docs](https://example.com/docs) for more",
			want: "see the docs\nfor more",
		},
		{
			name: "a bare link label falls back to its target",
			body: "![](https://example.com/img.png)",
			want: "https://exam\nple.com/img.\npng",
		},
		{
			name: "html entities decode and comments vanish",
			body: "<!-- hidden -->\nA &amp; B &lt;ok&gt;",
			want: "A & B <ok>",
		},
		{
			name: "fenced code loses its fences and is cut, not wrapped",
			body: "```go\nfmt.Println(1)\n```",
			want: "  fmt.Printl",
		},
		{name: "fenced code that fits is kept whole", body: "```\nok()\n```", want: "  ok()"},
		{name: "quotes take a gutter", body: "> quoted", want: "│ quoted"},
		{name: "blank runs collapse to one line", body: "first\n\n\n\nsecond", want: "first\n\nsecond"},
		{
			name: "a wrapped list item hangs under its bullet",
			body: "- one two three four five six",
			want: "• one two\n  three four\n  five six",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			width := tt.width
			if width == 0 {
				width = 12
			}

			if got := render(t, tt.body, width, 0); got != tt.want {
				t.Errorf("Render(%q) =\n%s\nwant\n%s", tt.body, got, tt.want)
			}
		})
	}
}

// A body is wrapped by line, so a cell-count cap can miss a long one entirely.
// The line cap is what keeps a changelog from owning the pane.
func TestRenderCapsTheLineCount(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("a line of prose\n", 40)

	got := render(t, body, 60, 5)
	if lines := strings.Count(got, "\n") + 1; lines != 6 {
		t.Errorf("rendering is %d lines, want the 5 allowed plus the marker:\n%s", lines, got)
	}

	if !strings.Contains(got, "… 35 more lines") {
		t.Errorf("rendering does not say how much it dropped:\n%s", got)
	}
}
