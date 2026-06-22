package tui

import (
	"os/exec"
	"strings"
)

func editorCmd(editor, path string) *exec.Cmd {
	parts := strings.Fields(editor)
	args := append(parts[1:], path)
	return exec.Command(parts[0], args...)
}
