package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding
	Tab      key.Binding
	New      key.Binding
	Edit     key.Binding
	Pin      key.Binding
	Archive  key.Binding
	Delete   key.Binding
	Search   key.Binding
	Filter   key.Binding
	Clear    key.Binding
	Journal  key.Binding
	Quit     key.Binding
}

var keys = keyMap{
	Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
	Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
	PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
	Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
	Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch panel")),
	New:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Pin:      key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pin")),
	Archive:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
	Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Filter:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
	Clear:    key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "clear filter")),
	Journal:  key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "journal")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
