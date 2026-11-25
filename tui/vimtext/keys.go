package vimtext

// The key names the engine reads, as tea.KeyPressMsg.String spells them.
const (
	keyEnter     = "enter"
	keyEsc       = "esc"
	keyBackspace = "backspace"
	keyCtrlH     = "ctrl+h"
	keyCtrlJ     = "ctrl+j"
	keyCtrlN     = "ctrl+n"
	keyCtrlP     = "ctrl+p"
	keyCtrlR     = "ctrl+r"
	keyLeft      = "left"
	keyRight     = "right"
	keyUp        = "up"
	keyDown      = "down"
	keyHome      = "home"
	keyEnd       = "end"
)

// isOperator reports d, c, or y.
func isOperator(k string) bool { return k == "d" || k == "c" || k == "y" }

// takesChar reports a command whose next key is a character it reads: f, F,
// t, T, and r.
func takesChar(k string) bool {
	switch k {
	case "f", "F", "t", "T", "r":
		return true
	}

	return false
}
