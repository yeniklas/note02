package tui

import "github.com/yeniklas/note02/internal/model"

type notesLoadedMsg struct {
	notes []model.Note
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
