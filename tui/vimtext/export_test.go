package vimtext

// Place puts the cursor at a line and byte offset in a mode, for a test to
// start a key sequence from a known state.
func (f *Field) Place(line, col int, m Mode) {
	f.mode = m
	f.cur = pos{line, col}
	f.anchor = f.cur
	f.setWant()
}

// EditCmd is the command Edit hands to tea.ExecProcess and what answers once
// it exits, for a test to run without a program.
var EditCmd = editCmd
