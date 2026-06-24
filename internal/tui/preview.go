package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/note02/internal/model"
)

type previewModel struct {
	vp        viewport.Model
	width     int
	height    int
	markdown  bool
	note      *model.Note
	renderer  *glamour.TermRenderer
	tagColors map[string]string
}

func newPreviewModel(markdown bool) previewModel {
	return previewModel{markdown: markdown}
}

func (m *previewModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.vp.Width = w
	m.vp.Height = h - 2 // leave room for metadata bar + separator
	if m.note != nil {
		m.refreshContent()
	}
}

func (m *previewModel) setNote(note *model.Note) {
	m.note = note
	m.vp = viewport.New(m.width, max(1, m.height-2))
	m.refreshContent()
}

func (m *previewModel) refreshContent() {
	if m.note == nil {
		m.vp.SetContent("")
		return
	}
	content := m.note.Content
	if m.markdown && content != "" {
		if m.renderer == nil {
			r, err := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.width),
			)
			if err == nil {
				m.renderer = r
			}
		}
		if m.renderer != nil {
			rendered, err := m.renderer.Render(content)
			if err == nil {
				content = rendered
			}
		}
	}
	m.vp.SetContent(content)
}

func (m *previewModel) view(focused bool) string {
	if m.note == nil {
		return styleMuted.Render("select a note to preview")
	}

	meta := m.renderMeta()
	sep := strings.Repeat("─", m.width)
	if !focused {
		sep = styleMuted.Render(sep)
	}

	return fmt.Sprintf("%s\n%s\n%s", meta, sep, m.vp.View())
}

func (m *previewModel) renderMeta() string {
	if m.note == nil {
		return ""
	}
	date := styleMuted.Render(m.note.UpdatedAt.Format("2006-01-02 15:04"))
	tags := ""
	if len(m.note.Tags) > 0 {
		var styled []string
		for _, t := range m.note.Tags {
			styled = append(styled, tagStyleFor(t, m.tagColors).Render(t))
		}
		tags = "  " + strings.Join(styled, styleMuted.Render(" · "))
	}
	title := ""
	if m.note.Title != "" {
		title = lipgloss.NewStyle().Bold(true).Render(m.note.Title) + "  "
	}
	return title + date + tags
}
