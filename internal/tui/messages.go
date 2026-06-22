package tui

import "github.com/yeniklas/note02/internal/model"

type notesLoadedMsg struct {
	notes []model.Note
}

// loadStartMsg signals that the note IDs have been enumerated and per-note
// decryption is about to begin.
type loadStartMsg struct {
	ids []string
}

// noteLoadedMsg carries a single decrypted note during startup loading.
type noteLoadedMsg struct {
	note model.Note
}

type noteSavedMsg struct {
	note    model.Note
	gitMsg  string // commit message to use for the git sync
}

type noteDeletedMsg struct {
	id     string
	gitMsg string
}

type repoStatusMsg struct {
	state syncState
	err   error
}

type errMsg struct {
	err error
}
