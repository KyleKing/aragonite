// Package glyph is the width policy these tools hold every drawn character to,
// and the normalizer that brings text from outside (a comment, a reaction, a
// title) inside it.
//
// A frame lines up only if the program and the terminal agree on how many cells
// each character advances the cursor. lipgloss, go-runewidth, and tmux agree on
// almost everything; where they part, or where a font draws a glyph wider than
// its cell, a row's borders and right column move. Five classes do it:
//
//   - TextEmoji: a code point Unicode marks Emoji with a text default (⚠ ▶ ✔ ⬇).
//     One cell to every width function, and drawn two wide by a terminal whose
//     font for it is an emoji font
//   - Presented: a cluster carrying VS16 (a red heart). Two cells to lipgloss and to tmux
//     3.4, one to a terminal without grapheme clustering (mode 2027)
//   - Joined: a ZWJ sequence (a woman and a laptop joined). One glyph of two cells where clustering is
//     on, each part's width summed where it is not
//   - Private: a private-use code point, which is every Nerd Font icon. One cell
//     everywhere, drawn two wide by a proportional Nerd Font or a fallback
//   - Recent: an emoji added after Unicode 9 (🩺, 🟠, 🥲). Two cells to lipgloss,
//     whose tables are current, and one to a terminal or tmux still measuring
//     by Unicode 9, which is WezTerm's default
//   - Wide: two cells (an emoji from Unicode 9 or before, a CJK ideograph).
//     Every width function and tmux agree. Chrome does without it all the same
//
// Chrome (the screen's own marks) uses only Stable glyphs. Content is passed
// through Settle, which leaves Wide alone and rewrites the others into
// something every terminal measures the same.
package glyph

import (
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// Class is how settled a grapheme cluster's width is.
type Class int

// The classes, settled first.
const (
	Stable Class = iota
	Wide
	TextEmoji
	Presented
	Joined
	Private
	Recent
)

func (c Class) String() string {
	switch c {
	case Stable:
		return "stable"
	case Wide:
		return "wide"
	case TextEmoji:
		return "a text-default emoji, drawn two cells wide by an emoji font"
	case Presented:
		return "an emoji presentation selector (VS16), two cells or one depending on the terminal"
	case Joined:
		return "a joined emoji sequence (ZWJ), one glyph or several depending on the terminal"
	case Private:
		return "a private-use (Nerd Font) glyph, drawn two cells wide by a proportional font"
	case Recent:
		return "an emoji newer than Unicode 9, one cell to a terminal measuring by Unicode 9"
	}

	return "unknown"
}

const (
	vs16     = '\U0000FE0F'
	vs15     = '\U0000FE0E'
	zwj      = '\U0000200D'
	keycap   = '\U000020E3'
	puaFirst = 0xE000
	puaLast  = 0xF8FF
	puaPlane = 0xF0000
	toneLow  = 0x1F3FB
	toneHigh = 0x1F3FF
	tagLow   = 0xE0020
	tagHigh  = 0xE007F
	riLow    = 0x1F1E6
	riHigh   = 0x1F1FF
)

// Of classifies one grapheme cluster. A cluster in more than one class
// answers the least settled.
func Of(cluster string) Class {
	switch {
	case strings.ContainsRune(cluster, zwj):
		return Joined
	case strings.ContainsRune(cluster, vs16):
		return Presented
	}

	r, _ := utf8.DecodeRuneInString(cluster)

	switch {
	case r >= puaFirst && r <= puaLast || r >= puaPlane:
		return Private
	case strings.ContainsRune(textEmoji, r) && !strings.ContainsRune(cluster, vs15):
		return TextEmoji
	case recent(r):
		return Recent
	case uniseg.StringWidth(cluster) > 1:
		return Wide
	}

	return Stable
}

// Each calls f with every grapheme cluster in s that is not Stable and the
// cell it starts at, stopping when f answers false.
func Each(s string, f func(cluster string, col int, c Class) bool) {
	state, col := -1, 0

	for s != "" {
		var cluster string

		var w int

		cluster, s, w, state = uniseg.FirstGraphemeClusterInString(s, state)
		if c := Of(cluster); c != Stable && !f(cluster, col, c) {
			return
		}

		col += w
	}
}

// Settle rewrites text from outside the program so every terminal measures it
// as lipgloss does. A text-default emoji becomes the width-stable mark it
// stands for where there is one (⚠ ▲, ✔ ✓, ▶ ▸, ⬇ ↓, ❤ ♡) and loses its
// selector where there is not; a joined sequence keeps its first emoji; skin
// tones, tags, and presentation selectors are dropped; a flag becomes its two
// letters; a private-use glyph becomes U+FFFD; an emoji newer than Unicode 9
// becomes ◌, since its width depends on the terminal's tables. Wide characters
// from Unicode 9 and before, emoji and CJK, are left alone: every width
// function and tmux count them as two cells. A terminal reporting grapheme
// clustering (mode 2027) outside tmux would measure all of this as lipgloss
// does, but tmux answers for itself rather than for the terminal, so these
// tools settle always.
func Settle(s string) string {
	if isASCII(s) {
		return s
	}

	var b strings.Builder

	state := -1
	for s != "" {
		var cluster string

		cluster, s, _, state = uniseg.FirstGraphemeClusterInString(s, state)
		b.WriteString(settleCluster(cluster))
	}

	return b.String()
}

func settleCluster(cluster string) string {
	r, size := utf8.DecodeRuneInString(cluster)

	if r >= riLow && r <= riHigh {
		var flag strings.Builder

		for _, ri := range cluster {
			if ri >= riLow && ri <= riHigh {
				flag.WriteRune('A' + ri - riLow)
			}
		}

		return flag.String()
	}

	if len(cluster) > size {
		cluster = bare(cluster)
	}

	switch Of(cluster) {
	case Private:
		return "\uFFFD"
	case Recent:
		return placeholder
	case TextEmoji:
		if stand, ok := standIns[cluster]; ok {
			return stand
		}
	case Stable, Wide, Presented, Joined:
	}

	return cluster
}

// bare is a cluster's first emoji without what modifies it: the parts a ZWJ
// joins after it, selectors, skin tones, tags, and a keycap's combiner.
func bare(cluster string) string {
	if i := strings.IndexRune(cluster, zwj); i >= 0 {
		cluster = cluster[:i]
	}

	return strings.Map(func(c rune) rune {
		switch {
		case c == vs16, c == vs15, c == keycap, c >= toneLow && c <= toneHigh, c >= tagLow && c <= tagHigh:
			return -1
		}

		return c
	}, cluster)
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}
