package pen_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/pen"
)

func TestPenDrawsWhatRenderDraws(t *testing.T) {
	t.Parallel()

	styles := map[string]lipgloss.Style{
		"plain":         lipgloss.NewStyle(),
		"foreground":    lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8800")),
		"background":    lipgloss.NewStyle().Background(lipgloss.Color("22")),
		"bold faint":    lipgloss.NewStyle().Bold(true).Faint(true).Foreground(lipgloss.Color("5")),
		"italic":        lipgloss.NewStyle().Italic(true),
		"reverse":       lipgloss.NewStyle().Reverse(true),
		"underline":     lipgloss.NewStyle().Underline(true),
		"strikethrough": lipgloss.NewStyle().Strikethrough(true),
		"width":         lipgloss.NewStyle().Width(12).Background(lipgloss.Color("1")),
		"max width":     lipgloss.NewStyle().MaxWidth(3),
		"padding":       lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("2")),
		"border":        lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
		"tab width":     lipgloss.NewStyle().TabWidth(2).Foreground(lipgloss.Color("3")),
		"tabs kept":     lipgloss.NewStyle().TabWidth(lipgloss.NoTabConversion),
		"transform":     lipgloss.NewStyle().Transform(strings.ToUpper),
		"link":          lipgloss.NewStyle().Hyperlink("https://example.com"),
		"inherited":     lipgloss.NewStyle().Bold(true).Inherit(lipgloss.NewStyle().Background(lipgloss.Color("4"))),
	}

	texts := []string{"", "x", "plain text", "\tindented", "a  b", "multi\nline", "crlf\r\nend", "naïve ✓ ü"}

	for name, style := range styles {
		p := pen.Of(style)

		for _, text := range texts {
			want := style.Render(text)

			if got := p.Render(text); got != want {
				t.Errorf("%s: Render(%q) = %q, want %q", name, text, got, want)
			}

			var b strings.Builder

			p.Write(&b, text)

			if b.String() != want {
				t.Errorf("%s: Write(%q) = %q, want %q", name, text, b.String(), want)
			}
		}
	}
}

func TestPenSpacesCarryTheStyleToAnyWidth(t *testing.T) {
	t.Parallel()

	for _, style := range []lipgloss.Style{
		lipgloss.NewStyle().Background(lipgloss.Color("22")),
		lipgloss.NewStyle().Underline(true),
		lipgloss.NewStyle(),
	} {
		p := pen.Of(style)

		for _, n := range []int{0, 1, 7, 128, 300} {
			var b strings.Builder

			p.Spaces(&b, n)

			want := ""
			if n > 0 {
				want = style.Render(strings.Repeat(" ", n))
			}

			if b.String() != want {
				t.Errorf("Spaces(%d) = %q, want %q", n, b.String(), want)
			}
		}
	}
}

func TestZeroPenDrawsUnstyled(t *testing.T) {
	t.Parallel()

	var p pen.Pen

	for _, text := range []string{"", "a\tb", "two\nlines"} {
		if got, want := p.Render(text), lipgloss.NewStyle().Render(text); got != want {
			t.Errorf("zero Pen Render(%q) = %q, want %q", text, got, want)
		}
	}
}

var sink string

func benchStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Background(lipgloss.Color("#1e2030")).Bold(true)
}

func BenchmarkStyleRender(b *testing.B) {
	style := benchStyle()

	for b.Loop() {
		sink = style.Render("handle(ctx context.Context)")
	}
}

func BenchmarkPenRender(b *testing.B) {
	p := pen.Of(benchStyle())

	for b.Loop() {
		sink = p.Render("handle(ctx context.Context)")
	}
}

func BenchmarkPenWrite(b *testing.B) {
	p := pen.Of(benchStyle())

	var out strings.Builder

	for b.Loop() {
		out.Reset()
		p.Write(&out, "handle(ctx context.Context)")
	}

	sink = out.String()
}
