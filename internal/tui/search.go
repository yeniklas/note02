package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type searchModel struct {
	input textinput.Model
}

func newSearchModel() searchModel {
	ti := textinput.New()
	ti.Placeholder = "search notes…"
	ti.CharLimit = 200
	return searchModel{input: ti}
}

func (m *searchModel) focus() {
	m.input.Focus()
}

func (m *searchModel) blur() {
	m.input.Blur()
}

func (m *searchModel) value() string {
	return m.input.Value()
}

func (m *searchModel) reset() {
	m.input.SetValue("")
}

func (m *searchModel) update(msg tea.Msg) (searchModel, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return *m, cmd
}

func (m *searchModel) view(width int) string {
	inner := styleBorder.Render("/ " + m.input.View())
	return inner
}
