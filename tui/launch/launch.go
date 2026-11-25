// Package launch starts a Bubble Tea program before it knows the terminal's background, and keeps
// the keys typed meanwhile.
//
// A key typed as the command is entered is already in the terminal's input queue when the program
// starts, and two things drop it. A background query run before the program
// (lipgloss.HasDarkBackground, which theme.Detect calls) reads the input itself and discards
// everything that is not its reply, for up to four seconds on a terminal that answers nothing. And a
// key that reaches the model before its first data load acts on an empty model.
//
// [Model] answers the first: it asks for the background (OSC 11) and the device attributes (DA1)
// through the program's own input, holds every message until the answer, then builds the real model
// for that background and replays what it held, in order. [Keys] answers the second: a model holds
// the key presses that arrive before its data has landed and replays them once it has. ctrl+c is
// never held by it.
package launch

import (
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Settle is how long a terminal has to answer before it is taken as dark. Every terminal in use
// answers the device-attributes query that follows the background one, which ends the wait at once;
// only a terminal that answers nothing waits this out.
const Settle = 2 * time.Second

// keyQuit is the key that ends a launch at once and is never held by [Keys].
const keyQuit = "ctrl+c"

// settledMsg is the end of the wait for an answer.
type settledMsg struct{}

// Model holds what arrives before the background does, then becomes the model build makes. It is
// the model a program is started with; once built, the program's own model replaces it, so none of
// its methods is called again.
type Model struct {
	build  func(dark bool) tea.Model
	held   []tea.Msg
	settle time.Duration
}

// New is a launch that builds the program with build once the terminal says whether its background
// is dark. The build func runs on the program's update loop, so it may set package-level styles: nothing
// draws until it returns.
func New(build func(dark bool) tea.Model) Model {
	return Model{build: build, settle: Settle}
}

// WithSettle is m waiting d for an answer instead of [Settle].
func (m Model) WithSettle(d time.Duration) Model {
	m.settle = d

	return m
}

// Init asks for the background and then for the device attributes in one write, so they are
// answered in that order: a terminal that answers the second and not the first has no background to
// report.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Raw(ansi.RequestBackgroundColor+ansi.RequestPrimaryDeviceAttributes),
		tea.Tick(m.settle, func(time.Time) tea.Msg { return settledMsg{} }),
	)
}

// Update builds the program on the first answer, or on the wait running out, and holds everything
// else until then. A ctrl+c does not wait out a silent terminal: it builds the program dark at once
// and is handed to it with the rest.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		return m.start(msg.IsDark())
	case uv.PrimaryDeviceAttributesEvent, settledMsg:
		return m.start(true)
	case tea.KeyPressMsg:
		if msg.String() == keyQuit {
			m.held = append(m.held, msg)

			return m.start(true)
		}
	}

	m.held = append(m.held, msg)

	return m, nil
}

// start builds the program and hands it, in order, what arrived while it was being asked for.
func (m Model) start(dark bool) (tea.Model, tea.Cmd) {
	program := m.build(dark)
	first := program.Init()

	program, cmd := Replay(program, m.held)

	return program, tea.Batch(first, cmd)
}

// View is the screen while the terminal is asked: nothing, on the alternate screen the program draws
// on, so the shell's screen does not flash between the two.
func (Model) View() tea.View {
	view := tea.NewView("")
	view.AltScreen = true

	return view
}

// Replay hands msgs to m one after another, in order, and batches the commands they answer.
func Replay[T tea.Msg](m tea.Model, msgs []T) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, len(msgs))

	for _, msg := range msgs {
		var cmd tea.Cmd

		m, cmd = m.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
