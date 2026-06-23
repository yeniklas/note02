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

// popupWidth is the fixed content width of the filter box, so it renders as a
// stable box regardless of tag lengths or scroll position.
const popupWidth = 28

func (m *filterPopupModel) view(activeTag string) string {
	box := styleBorder.Width(popupWidth)
	if len(m.tags) == 0 {
		return box.Render(styleMuted.Render("no tags"))
	}

	rows := m.visibleRows()
	end := min(m.offset+rows, len(m.tags))
	tree := tagTree(m.tags)
	var sb strings.Builder

	for i := m.offset; i < end; i++ {
		tag := m.tags[i]
		node := tree[i]

		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		indent := strings.Repeat("  ", node.depth)
		connector := ""
		if node.depth > 0 {
			connector = styleMuted.Render("└ ")
		}
		var label string
		if tag == activeTag {
			label = "• " + styleSelected.Render(node.label)
		} else {
			label = styleTag.Render(node.label)
		}
		sb.WriteString(cursor + indent + connector + label + "\n")
	}

	if end < len(m.tags) {
		sb.WriteString(styleMuted.Render(fmt.Sprintf("  ↓ %d more", len(m.tags)-end)))
	}

	return box.Render("Tags\n\n" + strings.TrimRight(sb.String(), "\n"))
}

// tagTreeRow describes how to render a tag within the hierarchy: its nesting
// depth (the number of ancestor tags that actually exist in the list) and the
// label to show — the segment beneath its nearest existing ancestor, or the
// full tag when it has no parent. This avoids implying a hierarchy that isn't
// there: a tag like "linux/dd" is only indented if "linux" is itself a tag.
type tagTreeRow struct {
	depth int
	label string
}

func tagTree(tags []string) []tagTreeRow {
	set := make(map[string]bool, len(tags))
	for _, t := range tags {
		set[t] = true
	}
	rows := make([]tagTreeRow, len(tags))
	for i, tag := range tags {
		segs := strings.Split(tag, "/")
		depth := 0
		nearest := ""
		prefix := ""
		for j := 0; j < len(segs)-1; j++ {
			if prefix == "" {
				prefix = segs[j]
			} else {
				prefix += "/" + segs[j]
			}
			if set[prefix] {
				depth++
				nearest = prefix
			}
		}
		label := tag
		if nearest != "" {
			label = tag[len(nearest)+1:]
		}
		rows[i] = tagTreeRow{depth: depth, label: label}
	}
	return rows
}
