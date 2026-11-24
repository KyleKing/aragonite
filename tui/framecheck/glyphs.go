package framecheck

import "strings"

// arm is a set of directions a box-drawing glyph reaches the edge of its cell in.
type arm uint8

const (
	up arm = 1 << iota
	down
	left
	right
)

// straight reports a glyph that only runs through its cell or stops in it: a
// rule, a side, or a half line. Those end wherever a design wants them to, so
// only a glyph that turns or branches has arms that must join something.
func (a arm) straight() bool {
	return a == up|down || a == left|right || a == up || a == down || a == left || a == right
}

var directions = []struct {
	name       string
	arm, back  arm
	dRow, dCol int
}{
	{name: "up", arm: up, back: down, dRow: -1},
	{name: "down", arm: down, back: up, dRow: 1},
	{name: "left", arm: left, back: right, dCol: -1},
	{name: "right", arm: right, back: left, dCol: 1},
}

// joins maps each box-drawing glyph to its arms. Weights and dashes join each
// other: a heavy scroll thumb in a light track is one line.
//
//nolint:gochecknoglobals // a lookup table
var joins = func() map[string]arm {
	m := map[string]arm{}
	add := func(glyphs string, a arm) {
		for _, r := range glyphs {
			m[string(r)] = a
		}
	}

	add("─━┄┅┈┉╌╍═", left|right)
	add("│┃┆┇┊┋╎╏║", up|down)
	add("┌┍┎┏╭╒╓╔", down|right)
	add("┐┑┒┓╮╕╖╗", down|left)
	add("└┕┖┗╰╘╙╚", up|right)
	add("┘┙┚┛╯╛╜╝", up|left)
	add("├┝┞┟┠┡┢┣╞╟╠", up|down|right)
	add("┤┥┦┧┨┩┪┫╡╢╣", up|down|left)
	add("┬┭┮┯┰┱┲┳╤╥╦", down|left|right)
	add("┴┵┶┷┸┹┺┻╧╨╩", up|left|right)
	add("┼┽┾┿╀╁╂╃╄╅╆╇╈╉╊╋╪╫╬", up|down|left|right)
	add("╴╸", left)
	add("╵╹", up)
	add("╶╺", right)
	add("╷╻", down)

	return m
}()

// emojiText is every code point below U+3300 that Unicode marks Emoji with a
// text default: one cell to every width function, and two in a terminal whose
// font for it is an emoji font, which is common for exactly these. Arrows
// U+2190-2193, the geometric shapes ▸ ◂ ◆ ● ◐, and box drawing are not in it.
const emojiText = "‼⁉™ℹ↔↕↖↗↘↙↩↪⌨⏏⏭⏮⏯⏱⏲⏸⏹⏺Ⓜ▪▫▶◀◻◼" +
	"☀☁☂☃☄☎☑☘☝☠☢☣☦☪☮☯☸☹☺♀♂♟♠♣♥♦♨♻♾⚒⚔⚕⚖⚗⚙⚛⚜⚠⚧⚰⚱⛈⛏⛑⛓⛩⛰⛱⛴⛷⛸⛹" +
	"✂✈✉✌✍✏✒✔✖✝✡✳✴❄❇❣❤➡⤴⤵⬅⬆⬇〰〽㊗㊙"

// emojiCapable reports a cluster that is a text-default emoji drawn without a
// presentation selector. One followed by VS16 asks for the emoji, which every
// width function here counts as two cells, and VS15 pins it to text.
func emojiCapable(cluster string) bool {
	r := []rune(cluster)
	if len(r) == 0 || len(r) > 1 {
		return false
	}

	return strings.ContainsRune(emojiText, r[0])
}
