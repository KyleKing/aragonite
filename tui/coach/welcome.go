package coach

import (
	"cmp"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/card"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/overlay"
	"github.com/kyleking/aragonite/tui/palette"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/table"
)

// WelcomeAction is the palette command that opens the welcome card again: ":welcome".
const WelcomeAction = ":welcome"

// The welcome card's bounds. Below WelcomeMinWidth by WelcomeMinHeight the card would cover the
// screen it introduces, so the tool shows WelcomeLine in its footer instead.
const (
	WelcomeMinWidth  = 60
	WelcomeMinHeight = 16
	// MaxStarters is the most starter keys a card lists: enough to start, few enough to read.
	MaxStarters = 6
)

const (
	welcomeMaxWidth = 64
	welcomeMargin   = 4
	welcomeKeyMin   = 6
	welcomeHalves   = 2
)

// Welcome is the card a tool shows on its first launch: a sentence on what the tool is for, the
// handful of keys that start a reader off, and where the rest are (`?` lists every key, `:` runs
// any command by name). Any key dismisses it and still does what it does, so the card never costs
// a keystroke, except Esc, which only dismisses. Dismissing records it in the ledger, so it shows
// once per reader; the `:welcome` palette command opens it again.
//
// The tool owns where it draws: Over centers the card over the screen's lines where the terminal
// is at least WelcomeMinWidth by WelcomeMinHeight, and below that the tool puts Line in its footer.
type Welcome struct {
	// Title is the card's top edge: "Welcome to jj-diff".
	Title string
	// Lead is a sentence or two on what the tool is for, wrapped inside the card.
	Lead string
	// Help and Palette are the keys that open the legend and the palette, "?" and ":" when empty.
	Help, Palette string
	// Starters are the keys to start with, at most MaxStarters drawn. StartersFrom fills them
	// from a keymap so a rebind moves them.
	Starters []keyhint.Hint

	open bool
}

// StartersFrom is the starter keys for actions, each labeled and worded from its binding in km so
// a rebind changes the card. An action km does not bind is left out.
func StartersFrom(km *keys.Keymap, actions ...string) []keyhint.Hint {
	out := make([]keyhint.Hint, 0, len(actions))

	for _, a := range actions {
		b, ok := km.Binding(a)
		if !ok || b.Label() == "" {
			continue
		}

		out = append(out, keyhint.Hint{Key: b.Label(), What: b.What})
	}

	return out
}

// HelpKeysFrom fills Help and Palette from km's help and palette bindings, so a rebind of either
// reaches the card's last lines.
func (w *Welcome) HelpKeysFrom(km *keys.Keymap) {
	if b, ok := km.Binding(keys.ActionHelp); ok && b.Label() != "" {
		w.Help = b.Label()
	}

	if b, ok := km.Binding(keys.ActionPalette); ok && b.Label() != "" {
		w.Palette = b.Label()
	}
}

// Due reports whether the ledger wants the welcome: its first session, and the card not yet
// dismissed. A nil ledger keeps nothing, so it is never due, which keeps a tool with no state
// directory from greeting its reader on every launch.
func Due(l *Ledger) bool {
	return l != nil && l.FirstRun() && !l.Welcomed()
}

// OpenIfDue opens the card when Due(l), and reports whether it did.
func (w *Welcome) OpenIfDue(l *Ledger) bool {
	if Due(l) {
		w.open = true
	}

	return w.open
}

// Open shows the card, which is what `:welcome` does.
func (w *Welcome) Open() { w.open = true }

// Showing reports whether the card is up.
func (w *Welcome) Showing() bool { return w != nil && w.open }

// Dismiss takes the card down and records in l that it has been seen.
func (w *Welcome) Dismiss(l *Ledger, at time.Time) {
	w.open = false
	l.MarkWelcomed(at)
}

// Key answers a key press while the card is up: it dismisses the card, and reports whether
// the key should still act, which every key but Esc does. With the card down every key passes.
func (w *Welcome) Key(l *Ledger, key string, at time.Time) bool {
	if !w.Showing() {
		return true
	}

	w.Dismiss(l, at)

	return key != "esc"
}

// WelcomeEntry is the palette entry for `:welcome`, which a tool appends to its palette-only
// commands.
func WelcomeEntry() palette.Entry {
	return palette.Entry{
		Action: WelcomeAction,
		What:   "show the welcome card again",
		Group:  keys.GroupGeneral,
		Also:   []string{"intro", "start", "tour", "keys", "help"},
		Prose: []string{
			"The card a first launch shows: the keys to start with, and where the rest are.",
		},
	}
}

// WelcomeFits reports whether a width by height terminal has room for the card.
func WelcomeFits(width, height int) bool {
	return width >= WelcomeMinWidth && height >= WelcomeMinHeight
}

func (w *Welcome) keys() (string, string) {
	return cmp.Or(w.Help, "?"), cmp.Or(w.Palette, ":")
}

func (w *Welcome) starters() []keyhint.Hint {
	return w.Starters[:min(len(w.Starters), MaxStarters)]
}

// Block is the card alone, as wide as width allows up to 64 cells less a margin: the title on the
// top edge, the lead, a row per starter key, the two lines on `?` and `:`, and "any key to start"
// on the bottom edge.
func (w *Welcome) Block(s skin.Skin, width int) []string {
	cw := max(1, min(welcomeMaxWidth, width-welcomeMargin))
	inner := card.InnerWidth(cw)
	help, pal := w.keys()
	starters := w.starters()

	keyw := welcomeKeyMin
	for _, h := range starters {
		keyw = max(keyw, lipgloss.Width(h.Key)+welcomeHalves)
	}

	keyw = max(keyw, lipgloss.Width(help)+welcomeHalves, lipgloss.Width(pal)+welcomeHalves)

	var body []string
	for _, l := range palette.Wrap(w.Lead, inner) {
		body = append(body, s.Body.Render(l))
	}

	if len(body) > 0 {
		body = append(body, "")
	}

	row := func(key, what string, face lipgloss.Style) string {
		return s.Key.Render(table.Pad(key, keyw, table.AlignLeft)) +
			face.Render(table.Truncate(what, max(1, inner-keyw)))
	}

	for _, h := range starters {
		body = append(body, row(h.Key, h.What, s.Body))
	}

	body = append(body, "",
		row(help, "every key, and why one is off here", s.Muted),
		row(pal, "find and run any command by name", s.Muted))

	c := card.Card{
		Title: s.Title.Render(w.Title),
		Foot:  s.Muted.Render("any key to start · esc closes"),
		Body:  body,
	}

	return c.Render(card.Styles{Edge: s.Edge}, cw)
}

// Over draws the card centered over base, a screen width by height cells whose rows are its
// lines, and returns base unchanged when the card is down or the terminal is too small for it (the
// tool shows Line in its footer then). The rows the card does not cover keep the screen.
func (w *Welcome) Over(s skin.Skin, base []string, width, height int) []string {
	if !w.Showing() || !WelcomeFits(width, height) {
		return base
	}

	block := w.Block(s, width)
	x := max(0, (width-lipgloss.Width(block[0]))/welcomeHalves)
	y := max(0, (height-len(block))/welcomeHalves)

	return overlay.Blit(base, block, x, y)
}

// Line is the card as one footer line, width cells wide at most, for a terminal too small for the
// card: "◇ welcome  ? keys  : commands  ]h next hunk  …  any key". It is empty when the card is
// down.
func (w *Welcome) Line(s skin.Skin, width int) string {
	if !w.Showing() || width < 1 {
		return ""
	}

	help, pal := w.keys()
	lead := s.Suggest.Render(Diamond+" welcome") + "  "
	tail := "  " + s.Muted.Render("any key")
	hints := append([]keyhint.Hint{
		{Key: help, What: "keys", Pin: true},
		{Key: pal, What: "commands", Pin: true},
	}, w.starters()...)

	room := width - lipgloss.Width(lead) - lipgloss.Width(tail)
	if room < 1 {
		return table.Truncate(Diamond+" welcome", width)
	}

	faces := keyhint.Styles{Key: s.Key, Text: s.Muted, Off: s.Muted, Bare: true}
	line := lead + keyhint.Fit(faces, hints, room) + tail

	return strings.TrimRight(line, " ")
}
