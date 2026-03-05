package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Enter    key.Binding
	Back     key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Attach   key.Binding
	Kill     key.Binding
	Delete   key.Binding
	Memorize     key.Binding
	MemorizeKeep key.Binding
	Search       key.Binding
	Help     key.Binding
	Quit     key.Binding
	Refresh      key.Binding
	CleanBroken  key.Binding
	CleanOld     key.Binding
	Rename       key.Binding
	Replay       key.Binding
}

var defaultKeyMap = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("k/up", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("j/down", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdn", "page down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "back"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next tab"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev tab"),
	),
	Attach: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "attach"),
	),
	Kill: key.NewBinding(
		key.WithKeys("K"),
		key.WithHelp("K", "kill"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Memorize: key.NewBinding(
		key.WithKeys("M"),
		key.WithHelp("M", "memorize & delete"),
	),
	MemorizeKeep: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "memorize (keep)"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	CleanBroken: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "clean broken"),
	),
	CleanOld: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "clean old"),
	),
	Rename: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "rename"),
	),
	Replay: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "replay"),
	),
}
