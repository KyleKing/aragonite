package coach

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/table"
)

// Layout is where a tool draws suggestions.
type Layout int

const (
	// Inline puts the next step in the status line and a tip in the footer's leading slot.
	Inline Layout = iota
	// Band draws a labeled "next" band above the footer, for a reader learning the tool.
	Band
)

// ParseLayout reads a config value: "inline" (or empty) or "band".
func ParseLayout(s string) (Layout, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "inline":
		return Inline, nil
	case "band":
		return Band, nil
	}

	return Inline, fmt.Errorf("%w: %q (want inline or band)", ErrLayout, s)
}

// ErrLayout is a config value naming no layout.
var ErrLayout = errors.New("unknown coach layout")

// The glyphs that keep a suggestion's meaning under NO_COLOR.
const (
	// Pointer marks a next step.
	Pointer = "▸"
	// Diamond marks a tip.
	Diamond = "◇"
)

// Line is the inline layout's status line for one next step, width cells wide: the tone, the
// state that prompts it, and the key that takes it, in the Suggest face.
// "▸ next  @ has changes but no description · d describe".
func Line(s skin.Skin, sg Suggestion, width int) string {
	head := s.Suggest.Render(Pointer+" "+sg.Tone.String()) + "  "
	tail := ""

	if sg.Key != "" {
		tail = s.Muted.Render(" · ") + s.Suggest.Render(sg.Key) + " " + s.Body.Render(sg.What)
	}

	room := width - lipgloss.Width(head) - lipgloss.Width(tail)
	if room < 1 {
		return ""
	}

	return head + s.Body.Render(table.Truncate(sg.Say, room)) + tail
}

// TipLine is a tip for the footer's leading slot, width cells wide: "◇ ]x jumps to the next
// conflict". It stays until the next key.
func TipLine(s skin.Skin, sg Suggestion, width int) string {
	lead := s.Suggest.Render(Diamond) + " "
	if sg.Key != "" {
		lead += s.Suggest.Render(sg.Key) + " "
	}

	room := width - lipgloss.Width(lead)
	if room < 1 {
		return ""
	}

	return lead + s.Muted.Render(table.Truncate(sg.Say, room))
}

// MaxBand is the most next steps the band lists before its tip.
const MaxBand = 3

const (
	bandKey  = 4
	bandWhat = 14
	bandCmd  = 3
)

// BandLines is the band layout, width cells wide: a labeled "next" rule, then a row per next step
// (its key and word, the state that prompts it, and the command it runs), then the tip if one is
// showing. It is nil when there is nothing to say, so the band takes no rows.
func BandLines(s skin.Skin, next []Suggestion, tip *Suggestion, width int) []string {
	next = next[:min(len(next), MaxBand)]
	if len(next) == 0 && tip == nil {
		return nil
	}

	out := []string{s.Rule("next", "", width)}

	for i := range next {
		out = append(out, bandRow(s, &next[i], width))
	}

	if tip != nil {
		lead := " " + s.Suggest.Render(Diamond) + " " + s.Muted.Render(table.Pad("tip", bandKey+bandWhat-1,
			table.AlignLeft)) + " "
		if tip.Key != "" {
			lead += s.Suggest.Render(tip.Key) + " "
		}

		out = append(out, lead+s.Muted.Render(table.Truncate(tip.Say, max(1, width-lipgloss.Width(lead)))))
	}

	return out
}

func bandRow(s skin.Skin, sg *Suggestion, width int) string {
	lead := " " + s.Suggest.Render(Pointer) + " " +
		s.Suggest.Render(table.Pad(sg.Key, bandKey, table.AlignLeft)) +
		s.Body.Render(table.Pad(table.Truncate(sg.What, bandWhat), bandWhat, table.AlignLeft))

	say := sg.Say
	if sg.Why != "" {
		say += " · " + sg.Why
	}

	room := width - lipgloss.Width(lead) - 1
	cmdw := 0

	if sg.Cmd != "" {
		cmdw = min(lipgloss.Width(sg.Cmd), room/bandCmd)
	}

	sayw := max(1, room-cmdw-1)
	row := lead + " " + s.Muted.Render(table.Pad(table.Truncate(say, sayw), sayw, table.AlignLeft))

	if cmdw > 0 {
		row += " " + s.Muted.Render(table.TruncateLeft(sg.Cmd, cmdw))
	}

	return row
}

// Mark sets Suggest on the hint whose key is the suggestion's, so the footer itself points at the
// next step: a conflict on screen brightens "x resolve".
func Mark(hints []keyhint.Hint, sg Suggestion) []keyhint.Hint {
	out := make([]keyhint.Hint, len(hints))
	copy(out, hints)

	for i := range out {
		if sg.Key != "" && out[i].Key == sg.Key {
			out[i].Suggest = true
		}
	}

	return out
}
