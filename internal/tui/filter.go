package tui

import (
	"fmt"
	"strings"
)

type filterPopupModel struct {
	tags   []string
	cursor int
	offset int
	height int
}

func newFilterPopupModel() filterPopupModel {
	return filterPopupModel{}
}

func (m *filterPopupModel) setTags(tags []string) {
	m.tags = tags
	m.cursor = 0
	m.offset = 0
}

func (m *filterPopupModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
	}
}

func (m *filterPopupModel) moveDown() {
	if m.cursor < len(m.tags)-1 {
		m.cursor++
		rows := m.visibleRows()
		if m.cursor >= m.offset+rows {
			m.offset = m.cursor - rows + 1
		}
	}
}

func (m *filterPopupModel) selected() string {
	if len(m.tags) == 0 || m.cursor >= len(m.tags) {
		return ""
	}
	return m.tags[m.cursor]
}

func (m *filterPopupModel) visibleRows() int {
	return max(1, m.height-4) // border + padding
}

func (m *filterPopupModel) view(activeTag string) string {
	if len(m.tags) == 0 {
		return styleBorder.Render(styleMuted.Render("no tags"))
	}

	rows := m.visibleRows()
	end := min(m.offset+rows, len(m.tags))
	var sb strings.Builder

	for i := m.offset; i < end; i++ {
		tag := m.tags[i]
		line := ""
		prefix := "  "
		if strings.Contains(tag, "/") {
			prefix = "    "
			line = styleMuted.Render("└ ") + styleTag.Render(tag)
		} else {
			line = styleTag.Render(tag)
		}
		if tag == activeTag {
			line = "• " + styleSelected.Render(tag)
			prefix = ""
		}
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		sb.WriteString(cursor + prefix + line + "\n")
	}

	if end < len(m.tags) {
		sb.WriteString(styleMuted.Render(fmt.Sprintf("  ↓ %d more", len(m.tags)-end)))
	}

	return styleBorder.Render("Tags\n\n" + strings.TrimRight(sb.String(), "\n"))
}
