package theme

import (
	"image/color"
	"io"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	colorful "github.com/lucasb-eyer/go-colorful"
)

// A terminal that cannot mix its own colors gets each background rebuilt for
// what it can show, rather than left to the renderer's nearest match.
//
// The nearest match is what makes a 256-color screen garish. The cube's darkest
// non-grey entries are fully saturated (22 is #005f00, 52 #5f0000, 17 #00005f),
// so a quiet green wash lands on either a grey or a saturated block, and a palette
// tuned to land off grey lands on the block. Delta, lazygit, and gitui avoid the
// cube for exactly this reason. So in 256 colors a background is a step on the
// grey ramp chosen by how far it sits from the frame's own background, and the
// side a line is on is said by the colored sign and number beside it. In 16 colors
// there is no grey ramp worth the name (8 is mid grey in most themes), so a
// background is dropped and the glyph and foreground carry the meaning alone.

// Lift is how many steps of the 256-color grey ramp a background role sits from
// the frame's background, toward the text. The ramp's steps are 10 levels of RGB
// apart, about the distance a truecolor tint here moves, so one step is a
// surface, two a wash, and four the deepest mark.
type Lift int

// The lifts the skin and the diff faces use, in ramp steps from the background.
const (
	Sunk     Lift = -1 // a strip under the frame: a pinned panel
	Flush    Lift = 0  // the frame's own background, painted
	Surface  Lift = 1  // a raised card
	Wash     Lift = 2  // a changed line, a tab, a file's heading
	Cursor   Lift = 3  // the row under the cursor
	Emphasis Lift = 4  // the run inside a changed line that differs, a badge
)

const (
	greyFirst  = 232 // xterm's grey ramp: 24 steps, 8 to 238 in RGB
	greySteps  = 24
	greyOrigin = 8
	greyStride = 10
	cubeWhite  = 231 // #ffffff, past the ramp's light end
	cubeBlack  = 16  // #000000, past its dark end
)

// Profile is the color profile a program drawing to output should fit its
// colors to: colorprofile.Detect, except that a COLORTERM of truecolor or 24bit
// is honored inside tmux and screen. Detect ignores it there and asks
// `tmux info` for the Tc or RGB feature instead, which a server with no client
// attached, or one whose terminal-features never named the outer terminal,
// does not report, so a laptop with millions of colors drew the 256-color
// greys. Inside tmux or screen a 16-color answer is lifted too, since TERM there
// is tmux's own (screen by default) rather than the terminal's. Outside them,
// and for a pipe or NO_COLOR, Detect's answer stands.
//
// A program hands the same answer to Bubble Tea (tea.WithColorProfile), so
// the depth the faces were fitted to is the depth the renderer draws at.
func Profile(output io.Writer, env []string) colorprofile.Profile {
	p := colorprofile.Detect(output, env)

	switch {
	case p == colorprofile.TrueColor || p <= colorprofile.ASCII || !millionsAsked(env):
		return p
	case p == colorprofile.ANSI256, multiplexed(env):
		// Inside tmux TERM is tmux's own (screen, unless default-terminal says
		// otherwise), which reads as 16 colors, while COLORTERM is the outer
		// terminal's, inherited. tmux passes RGB through when the outer
		// terminal's entry carries it (terminal-overrides ",xterm-256color:RGB").
		return colorprofile.TrueColor
	}

	return p
}

func millionsAsked(env []string) bool {
	v := strings.ToLower(lookup(env, "COLORTERM"))

	return v == "truecolor" || v == "24bit"
}

func multiplexed(env []string) bool {
	term := lookup(env, "TERM")

	return lookup(env, "TMUX") != "" || strings.HasPrefix(term, "tmux") || strings.HasPrefix(term, "screen")
}

func lookup(env []string, key string) string {
	for _, kv := range env {
		if name, value, _ := strings.Cut(kv, "="); name == key {
			return value
		}
	}

	return ""
}

// Ground fits a background to profile. TrueColor keeps c. ANSI256 replaces it
// with the grey that sits lift steps from base's own grey, toward the light on a
// dark base and toward the dark on a light one. ANSI and below answer
// lipgloss.NoColor, which draws no background at all.
func Ground(c, base color.Color, lift Lift, profile colorprofile.Profile) color.Color {
	switch profile {
	case colorprofile.TrueColor, colorprofile.Unknown:
		return c
	case colorprofile.ANSI256:
		return Grey(base, lift)
	default:
		return lipgloss.NoColor{}
	}
}

// Grey is the 256-color grey lift steps from the ramp step nearest base in
// lightness, toward the light on a dark base and toward the dark on a light one.
// Past either end of the ramp it is the cube's black or white.
func Grey(base color.Color, lift Lift) color.Color {
	step := greyStep(base)
	if !Dark(base) {
		lift = -lift
	}

	switch at := step + int(lift); {
	case at < 0:
		return ansi.IndexedColor(cubeBlack)
	case at >= greySteps:
		return ansi.IndexedColor(cubeWhite)
	default:
		return ansi.IndexedColor(greyFirst + at)
	}
}

// Dark reports whether c reads as a dark background: lightness under half.
func Dark(c color.Color) bool {
	const half = 0.5

	l, _, _ := lab(c)

	return l < half
}

// greyStep is the ramp step whose lightness is nearest c's.
func greyStep(c color.Color) int {
	want, _, _ := lab(c)
	best, dist := 0, math.Inf(1)

	for i := range greySteps {
		v := uint8(greyOrigin + i*greyStride)
		l, _, _ := lab(color.Gray{Y: v})

		if d := math.Abs(l - want); d < dist {
			best, dist = i, d
		}
	}

	return best
}

func lab(c color.Color) (float64, float64, float64) {
	cf, _ := colorful.MakeColor(c)

	return cf.Lab()
}

// cubeLevels are the channel values of xterm's 6x6x6 color cube (16-231).
var cubeLevels = [6]uint8{0, 95, 135, 175, 215, 255}

const (
	cubeFirst = 16
	cubeSide  = 6
	// A color under chromaFloor of Lab chroma is a grey of its own, and a grey
	// answer loses nothing.
	chromaFloor = 0.25
)

// Hue fits a foreground that carries its meaning by hue, such as a diff's sign
// or a count, to profile. The nearest match colorprofile makes in 256 colors weighs
// lightness heavily enough that a saturated mid-dark color can land on the grey
// ramp (Latte's red, #d20f39, answers 241), which erases the hue the color was
// chosen for. In 256 colors Hue answers the nearest non-grey entry of the color
// cube instead whenever that match would be a grey and c is not one; every other
// case keeps c for the renderer to convert.
func Hue(c color.Color, profile colorprofile.Profile) color.Color {
	if profile != colorprofile.ANSI256 || c == nil {
		return c
	}

	if _, a, b := lab(c); math.Hypot(a, b) < chromaFloor || !greyIndexed(profile.Convert(c)) {
		return c
	}

	return ansi.IndexedColor(nearestCube(c))
}

// greyIndexed reports whether c, a 256-color answer, is a grey: the ramp or a
// cube entry with equal channels.
func greyIndexed(c color.Color) bool {
	r, g, b, _ := c.RGBA()

	return r == g && g == b
}

// nearestCube is the cube index, never one of its greys, nearest c in Lab.
func nearestCube(c color.Color) uint8 {
	want, _ := colorful.MakeColor(c)
	best, dist := uint8(cubeFirst), math.Inf(1)

	for r := range uint8(cubeSide) {
		for g := range uint8(cubeSide) {
			for b := range uint8(cubeSide) {
				if r == g && g == b {
					continue
				}

				at := color.RGBA{R: cubeLevels[r], G: cubeLevels[g], B: cubeLevels[b], A: math.MaxUint8}
				got, _ := colorful.MakeColor(at)

				if d := want.DistanceLab(got); d < dist {
					best, dist = cubeFirst+r*cubeSide*cubeSide+g*cubeSide+b, d
				}
			}
		}
	}

	return best
}
