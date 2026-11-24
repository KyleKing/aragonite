package trail

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/table"
)

// Glyphs a row starts with, so NO_COLOR keeps whether a call failed.
const (
	okMark   = "✓"
	failMark = "✗"
	readMark = "·"
)

// Echo is a status line's account of a finished write: what it did, the command it ran, and how to
// reverse it, so every write teaches its command line: "squashed into rnzx · jj squash --from
// wkwy --into rnzx · u undo". Empty parts are left out.
func Echo(did string, c *Call, undo string) string {
	parts := slices.DeleteFunc([]string{did, c.Shell(), undo}, func(p string) bool { return p == "" })

	return strings.Join(parts, " · ")
}

// Lines draws calls as a log list width cells wide, newest first as Calls answers them, the row at
// sel in the selection face: a mark for how it ended, the time it started, the shell line, how
// long it took, and for a failure the line of stderr that says why under it.
func Lines(s skin.Skin, calls []Call, width, sel int) []string {
	out := make([]string, 0, len(calls))

	for i := range calls {
		out = append(out, row(s, &calls[i], width, i == sel)...)
	}

	return out
}

const (
	clockWidth = 5
	tookWidth  = 6
	gaps       = 3
	markWidth  = 2
)

func row(s skin.Skin, c *Call, width int, selected bool) []string {
	face := func(st lipgloss.Style) lipgloss.Style {
		if selected {
			return st.Inherit(s.Selection)
		}

		return st
	}

	mark, markFace := okMark, s.Success
	switch {
	case c.Failed():
		mark, markFace = failMark, s.Error
	case !c.Write:
		mark, markFace = readMark, s.Muted
	}

	cmdFace := s.Body
	if !c.Write {
		cmdFace = s.Muted
	}

	room := max(1, width-markWidth-clockWidth-tookWidth-gaps)
	line := face(markFace).Render(mark+" ") +
		face(s.Muted).Render(c.Start.Format("15:04")+" ") +
		face(cmdFace).Render(table.Pad(table.Truncate(c.Shell(), room), room, table.AlignLeft)) +
		face(s.Muted).Render(" "+table.Pad(took(c.Took), tookWidth, table.AlignRight))

	if pad := width - lipgloss.Width(line); pad > 0 {
		line += face(lipgloss.NewStyle()).Render(strings.Repeat(" ", pad))
	}

	out := []string{line}

	if c.Failed() {
		why := "exit " + strconv.Itoa(c.Exit)
		if said := c.Said(); said != "" {
			why += " · " + said
		}

		lead := strings.Repeat(" ", markWidth+clockWidth+1)
		out = append(out, lead+s.Error.Render(table.Truncate(why, max(1, width-len(lead)))))
	}

	return out
}

// took is a duration as the log's last column says it: "40ms", "1.2s", "3m".
func took(d time.Duration) string {
	switch {
	case d < time.Second:
		return strconv.FormatInt(d.Milliseconds(), 10) + "ms"
	case d < time.Minute:
		return strconv.FormatFloat(d.Seconds(), 'f', 1, 64) + "s"
	}

	return strconv.Itoa(int(d.Minutes())) + "m"
}
