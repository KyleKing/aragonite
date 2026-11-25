package vimtext_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/framecheck"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
	"github.com/kyleking/aragonite/tui/vimtext"
)

var update = flag.Bool("update", false, "rewrite the golden frames")

func stripped(s string) string { return ansi.Strip(s) }

// golden compares got with testdata/<name>.golden, rewriting it under -update.
func golden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	if got != string(want) {
		t.Errorf("%s differs from its golden (go test ./tui/vimtext -update):\n%s", name, got)
	}
}

//nolint:gosmopolitan // CJK is the point: wide characters wrap and draw two cells
const prose = "Fix the race in the read marks: two panes wrote the same file at once, " +
	"and the second write dropped the first one's marks.\n\nRefs: 日本語 and naïve café\n- one\n- two"

// frames are the field's modes at 80 columns, each as the keys that reach it.
var frames = []struct {
	name string
	keys string
	opts []vimtext.Option
}{
	{name: "insert", keys: ""},
	{name: "normal", keys: "<Esc>gg"},
	{name: "visual", keys: "<Esc>gg2wve"},
	{name: "visual-line", keys: "<Esc>gg2jVj"},
	{name: "pending", keys: "<Esc>gg2d"},
	{name: "exline", keys: "<Esc>:wq"},
	{name: "plain", keys: "", opts: []vimtext.Option{vimtext.WithVim(false)}},
	{name: "one-line", keys: "<Esc>0", opts: []vimtext.Option{vimtext.WithOneLine(), vimtext.WithPrompt("name: ")}},
	{name: "scrolled", keys: "<Esc>G", opts: []vimtext.Option{vimtext.WithHeight(1, 3)}},
}

func sk(profile colorprofile.Profile) *skin.Skin {
	p := theme.Macchiato()
	s := skin.For(p, p.Mauve, profile)

	return &s
}

// field opens a field on prose in a frame's mode.
func field(t *testing.T, profile colorprofile.Profile, keys string, opts ...vimtext.Option) *vimtext.Field {
	t.Helper()

	opts = append([]vimtext.Option{
		vimtext.WithTitle("describe qpvx"), vimtext.WithStyles(vimtext.StylesFor(sk(profile))),
		vimtext.WithEditor("t-*.md"),
	}, opts...)
	f := vimtext.New(prose, opts...)
	press(t, f, keys)

	return f
}

// downsample is what a terminal of profile is sent once the renderer fits the
// frame to it, as tea.WithColorProfile does.
func downsample(t *testing.T, frame string, profile colorprofile.Profile) string {
	t.Helper()

	var b bytes.Buffer

	w := &colorprofile.Writer{Forward: &b, Profile: profile}
	if _, err := w.WriteString(frame); err != nil {
		t.Fatal(err)
	}

	return b.String()
}

// TestFrames pins each mode at 80 columns, after
// framecheck holds every row to the width and the glyph policy.
func TestFrames(t *testing.T) {
	t.Parallel()

	for _, fr := range frames {
		t.Run(fr.name, func(t *testing.T) {
			t.Parallel()

			frame := field(t, colorprofile.TrueColor, fr.keys, fr.opts...).Render(80)
			framecheck.Require(t, fr.name, frame, framecheck.Width(80))
			// The readable frame first, then the same frame with its escapes,
			// which is where the cursor and the selection show.
			golden(t, fr.name, stripped(frame)+"\n\n"+frame+"\n")
		})
	}
}

// TestFrames_Profiles pins the visual frame's escapes at each color depth: a
// tint and filled badges in truecolor and 256 colors, reverse video for the
// selection at 16 colors and under NO_COLOR, where only attributes are left.
func TestFrames_Profiles(t *testing.T) {
	for _, c := range []struct {
		name    string
		noColor string
		profile colorprofile.Profile
	}{
		{"truecolor", "", colorprofile.TrueColor},
		{"256", "", colorprofile.ANSI256},
		{"16", "", colorprofile.ANSI},
		{"no-color", "1", colorprofile.Ascii},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", c.noColor)

			f := field(t, c.profile, "<Esc>gg2wve")
			frame := downsample(t, f.Render(80), c.profile)
			framecheck.Require(t, c.name, frame, framecheck.Width(80))

			if c.profile <= colorprofile.ANSI && !strings.Contains(frame, "\x1b[7m") {
				t.Errorf("the selection is not reversed at %s", c.name)
			}

			golden(t, "visual-"+c.name, frame+"\n")
		})
	}
}

// TestView_WrapsAndScrolls: every row is the field's width, a long line wraps
// at a space, the cursor stays in sight across wrapped lines, and the field
// grows from its minimum to its maximum.
func TestView_WrapsAndScrolls(t *testing.T) {
	t.Parallel()

	for _, width := range []int{12, 20, 33, 80} {
		//nolint:gosmopolitan // CJK is the point: a wide character must not split across rows
		f := vimtext.New(strings.Repeat("word 日本 ", 40), vimtext.WithHeight(2, 4))

		for _, seq := range []string{"", "<Esc>0", "<Esc>$", "<Esc>gg0w", "<Esc>G$bbbb"} {
			press(t, f, seq)

			view := f.View(width)
			framecheck.Require(t, "view", view, framecheck.Width(width))

			if rows := strings.Count(view, "\n") + 1; rows != 4 {
				t.Fatalf("%d rows at width %d", rows, width)
			}

			if !strings.Contains(view, "\x1b[7m") {
				t.Fatalf("the cursor is off screen at width %d after %q:\n%s", width, seq, view)
			}
		}
	}

	short := vimtext.New("hi", vimtext.WithHeight(2, 4))
	if h := short.Height(40); h != 2 {
		t.Fatalf("a short field is %d rows", h)
	}
}

// TestView_Placeholder draws the placeholder dimmed under the cursor in an empty
// field.
func TestView_Placeholder(t *testing.T) {
	t.Parallel()

	f := vimtext.New("", vimtext.WithPlaceholder("bookmark name"), vimtext.WithPrompt("> "))
	view := f.View(30)
	framecheck.Require(t, "placeholder", view, framecheck.Width(30))

	if got := stripped(view); got != "> bookmark name"+strings.Repeat(" ", 15) {
		t.Fatalf("placeholder %q", got)
	}
}

// TestView_OneLineScrollsSideways keeps the cursor in sight on a line wider than
// the field.
func TestView_OneLineScrollsSideways(t *testing.T) {
	t.Parallel()

	f := vimtext.New(strings.Repeat("abcdefghij", 10)+"END", vimtext.WithOneLine())

	view := stripped(f.View(20))
	if !strings.HasSuffix(strings.TrimRight(view, " "), "END") || len(view) != 20 {
		t.Fatalf("at the end: %q", view)
	}

	press(t, f, "<Esc>0")

	if view := stripped(f.View(20)); !strings.HasPrefix(view, "abcdefghij") {
		t.Fatalf("at the start: %q", view)
	}
}
