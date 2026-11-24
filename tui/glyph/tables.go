package glyph

// textEmoji is every code point below U+3300 that Unicode marks Emoji with a
// text default (emoji-data.txt: Emoji=Yes, Emoji_Presentation=No), less the
// ASCII digits, #, and *. Arrows U+2190-2193, the geometric shapes ▸ ◂ ◆ ● ◐,
// and box drawing are not in it.
const textEmoji = "©®‼⁉™ℹ↔↕↖↗↘↙↩↪⌨⏏⏭⏮⏯⏱⏲⏸⏹⏺Ⓜ▪▫▶◀◻◼" +
	"☀☁☂☃☄☎☑☘☝☠☢☣☦☪☮☯☸☹☺♀♂♟♠♣♥♦♨♻♾⚒⚔⚕⚖⚗⚙⚛⚜⚠⚧⚰⚱⛈⛏⛑⛓⛩⛰⛱⛴⛷⛸⛹" +
	"✂✈✉✌✍✏✒✔✖✝✡✳✴❄❇❣❤➡⤴⤵⬅⬆⬇〰〽㊗㊙"

// standIns is the width-stable mark each common text-default emoji is drawn as
// when it arrives in content: what it means, in a code point no terminal draws
// from an emoji font.
//
//nolint:gochecknoglobals // a lookup table
var standIns = map[string]string{
	"⚠": "▲", "✔": "✓", "✖": "✗", "☑": "☒", "▶": "▸", "◀": "◂",
	"⬆": "↑", "⬇": "↓", "⬅": "←", "➡": "→", "↩": "↵", "↪": "↳",
	"❤": "♡", "♥": "♡", "✏": "✎", "⏺": "●", "▪": "■", "▫": "□",
	"◼": "■", "◻": "□", "⏸": "‖", "✳": "✱", "✴": "✶", "❄": "✻",
	"™": "TM", "©": "(c)", "®": "(R)", "‼": "!!", "⁉": "!?", "ℹ": "i",
}

// placeholder stands in for an emoji whose width the terminal may not know.
const placeholder = "\u25CC"

// recentRanges are the emoji blocks Unicode 10 through 15 filled, which a
// terminal measuring by Unicode 9 (WezTerm's default, and tmux built against an
// older libc) counts as one cell.
//
//nolint:gochecknoglobals // a lookup table
var recentRanges = [][2]rune{
	{0x1F6D5, 0x1F6DF},
	{0x1F6F7, 0x1F6FF},
	{0x1F7E0, 0x1F7FF},
	{0x1F90C, 0x1F90F},
	{0x1F91F, 0x1F91F},
	{0x1F928, 0x1F92F},
	{0x1F931, 0x1F932},
	{0x1F93F, 0x1F93F},
	{0x1F94C, 0x1F94F},
	{0x1F95F, 0x1F97F},
	{0x1F992, 0x1F9BF},
	{0x1F9C1, 0x1F9FF},
	{0x1FA70, 0x1FAFF},
}

func recent(r rune) bool {
	for _, span := range recentRanges {
		if r >= span[0] && r <= span[1] {
			return true
		}
	}

	return false
}
