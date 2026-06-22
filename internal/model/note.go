package model

import (
	"sort"
	"time"
)

// PinnedTag is the tag that marks a note as pinned. Pinned notes sort to the top
// of the list. It is treated specially in the TUI: hidden from tag chips and the
// tag filter, and toggled via the pin keybinding.
const PinnedTag = "pinned"

type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsPinned reports whether the note carries the pin tag.
func (n Note) IsPinned() bool {
	for _, t := range n.Tags {
		if t == PinnedTag {
			return true
		}
	}
	return false
}

// SortNotes orders notes pinned-first, then most-recently-updated first within
// each group.
func SortNotes(notes []Note) {
	sort.Slice(notes, func(i, j int) bool {
		if notes[i].IsPinned() != notes[j].IsPinned() {
			return notes[i].IsPinned()
		}
		return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
	})
}
