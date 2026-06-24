package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/note02/internal/model"
)

type listModel struct {
	notes     []model.Note
	cursor    int
	width     int
	height    int
	offset    int
	loading   bool
	tagColors map[string]string
}

func newListModel() listModel {
	return listModel{loading: true}
}

func (m *listModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

// preferredWidth returns the minimum width needed to display all notes without
// truncating titles, capped at a reasonable maximum.
func (m *listModel) preferredWidth() int {
	const (
		cursorW = 2
		minW    = 24
	)
	best := minW
	for _, n := range m.notes {
		title := noteTitle(n)
		tags := strings.Join(visibleTags(n.Tags), " ")
		w := cursorW + len([]rune(title))
		if tags != "" {
			w += 1 + len(tags)
		}
		if w > best {
			best = w
		}
	}
	return best
}

func (m *listModel) setNotes(notes []model.Note) {
	m.notes = notes
	m.loading = false
	if m.cursor >= len(notes) {
		m.cursor = max(0, len(notes)-1)
	}
}

func (m *listModel) selected() *model.Note {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	return &m.notes[m.cursor]
}

func (m *listModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
	}
}

func (m *listModel) moveDown() {
	if m.cursor < len(m.notes)-1 {
		m.cursor++
		if m.cursor >= m.offset+m.visibleRows() {
			m.offset = m.cursor - m.visibleRows() + 1
		}
	}
}

func (m *listModel) pageUp() {
	m.cursor -= m.visibleRows()
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
}

func (m *listModel) pageDown() {
	m.cursor += m.visibleRows()
	if m.cursor > len(m.notes)-1 {
		m.cursor = max(0, len(m.notes)-1)
	}
	if m.cursor >= m.offset+m.visibleRows() {
		m.offset = m.cursor - m.visibleRows() + 1
	}
}

func (m *listModel) jumpTop() {
	m.cursor = 0
	m.offset = 0
}

func (m *listModel) jumpBottom() {
	m.cursor = max(0, len(m.notes)-1)
	if m.cursor >= m.visibleRows() {
		m.offset = m.cursor - m.visibleRows() + 1
	}
}

func (m *listModel) visibleRows() int {
	// header + scroll indicator
	return max(1, m.height-2)
}

func (m *listModel) view(focused bool, selected map[string]bool) string {
	if m.loading {
		return styleMuted.Render("loading…")
	}
	if len(m.notes) == 0 {
		return styleMuted.Render("no notes · press n to create one")
	}

	inner := m.width - 2 // account for right border
	if inner < 1 {
		inner = 1
	}

	rows := m.visibleRows()
	end := min(m.offset+rows, len(m.notes))
	var sb strings.Builder

	for i := m.offset; i < end; i++ {
		note := m.notes[i]
		pinned := note.IsPinned()
		isSel := selected[note.ID]
		var cursor string
		var row string

		switch {
		case i == m.cursor && isSel:
			cursor = "●>"
		case i == m.cursor:
			cursor = "> "
		case isSel:
			cursor = "● "
		default:
			cursor = "  "
		}

		shownTags := visibleTags(note.Tags)
		tags := coloredTags(shownTags, 18, m.tagColors)
		tagsW := lipgloss.Width(tags)

		// available space for title: cursor(2) + title + [space + tags]
		titleWidth := inner - 2
		if tagsW > 0 {
			titleWidth -= 1 + tagsW
		}
		if titleWidth < 1 {
			titleWidth = 1
		}
		title := truncate(noteTitle(note), titleWidth)

		line := fmt.Sprintf("%-*s %s", titleWidth, title, tags)

		if i == m.cursor || isSel {
			if focused {
				row = cursor + styleSelected.Render(line)
			} else {
				row = cursor + lipgloss.NewStyle().Bold(true).Render(line)
			}
		} else if pinned {
			row = cursor + stylePinned.Render(line)
		} else {
			row = cursor + line
		}
		sb.WriteString(row + "\n")
	}

	// scroll indicator
	if end < len(m.notes) {
		sb.WriteString(styleMuted.Render(fmt.Sprintf("  ↓ %d more", len(m.notes)-end)))
	}

	return sb.String()
}

func noteTitle(n model.Note) string {
	if n.Title != "" {
		return n.Title
	}
	lines := strings.SplitN(strings.TrimSpace(n.Content), "\n", 2)
	if len(lines) > 0 && lines[0] != "" {
		t := strings.TrimLeft(lines[0], "#> ")
		if t != "" {
			return t
		}
	}
	return "(untitled)"
}

func formatTags(tags []string, maxLen int) string {
	plain := plainTags(tags, maxLen)
	if plain == "" {
		return ""
	}
	return styleTag.Render(plain)
}

// coloredTags renders each tag with its configured color (falling back to the
// default tag style), respecting maxLen on the combined plain-text width.
func coloredTags(tags []string, maxLen int, tagColors map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	var parts []string
	used := 0
	for i, t := range tags {
		need := len([]rune(t))
		if i > 0 {
			need++ // space separator
		}
		if used+need > maxLen {
			// append truncation indicator to last part if any fit
			if len(parts) > 0 {
				parts[len(parts)-1] += "…"
			} else {
				// even the first tag doesn't fit; truncate it
				r := []rune(t)
				cut := maxLen - 1
				if cut < 1 {
					cut = 1
				}
				parts = append(parts, tagStyleFor(t, tagColors).Render(string(r[:cut])+"…"))
			}
			break
		}
		parts = append(parts, tagStyleFor(t, tagColors).Render(t))
		used += need
	}
	return strings.Join(parts, " ")
}

// plainTags joins tags space-separated and truncates to maxLen, without styling.
func plainTags(tags []string, maxLen int) string {
	if len(tags) == 0 {
		return ""
	}
	plain := strings.Join(tags, " ")
	if len(plain) > maxLen {
		plain = plain[:maxLen-1] + "…"
	}
	return plain
}

// visibleTags returns the note's tags minus the pin tag, which is signalled by
// the gold row styling instead of a tag chip.
func visibleTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t == model.PinnedTag {
			continue
		}
		out = append(out, t)
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
