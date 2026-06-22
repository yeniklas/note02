package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yeniklas/note02/internal/config"
	"github.com/yeniklas/note02/internal/git"
	"github.com/yeniklas/note02/internal/model"
	"github.com/yeniklas/note02/internal/store"
	"github.com/yeniklas/note02/internal/tui"
	"golang.org/x/term"
)

func main() {
	journalFlag := flag.Bool("journal", false, "open or create today's journal entry and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fatalf("config: %v", err)
	}
	if cfg.Repo.Path == "" {
		fatalf("repo.path is not set in ~/.config/note02/config.toml")
	}

	notesDir := filepath.Join(cfg.Repo.Path, "notes")
	if err := os.MkdirAll(notesDir, 0700); err != nil {
		fatalf("create notes dir: %v", err)
	}

	passphrase, err := readPassphrase()
	if err != nil {
		fatalf("passphrase: %v", err)
	}

	s := store.New(cfg.Repo.Path, passphrase)
	journalTags := cfg.Journal.EffectiveTags()

	if *journalFlag {
		if err := runJournal(s, cfg.Repo.Path, journalTags); err != nil {
			fatalf("journal: %v", err)
		}
		return
	}

	app := tui.New(s, cfg.Display.Markdown, journalTags)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fatalf("run: %v", err)
	}
}

func runJournal(s *store.Store, repoPath string, journalTags []string) error {
	title := "Journal " + time.Now().Format("2006-01-02")

	notes, err := s.List()
	if err != nil {
		return fmt.Errorf("load notes: %w", err)
	}

	var existing *model.Note
	for i := range notes {
		if notes[i].Title == title {
			existing = &notes[i]
			break
		}
	}

	var fileTitle string
	var fileTags []string
	var fileContent string
	if existing != nil {
		fileTitle = existing.Title
		fileTags = existing.Tags
		fileContent = existing.Content
	} else {
		fileTitle = title
		fileTags = journalTags
	}

	fm := fmt.Sprintf("---\ntitle: %s\ntags: %s\n---\n\n%s", fileTitle, strings.Join(fileTags, ", "), fileContent)

	tmp, err := os.CreateTemp("", "note02-journal-*.md")
	if err != nil {
		return err
	}
	if _, err := tmp.WriteString(fm); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], tmp.Name())...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmp.Name())
		return err
	}

	data, err := os.ReadFile(tmp.Name())
	os.Remove(tmp.Name())
	if err != nil {
		return err
	}

	fmTitle, tags, body := parseFrontmatter(strings.TrimRight(string(data), "\n"))
	body = strings.TrimRight(body, "\n")

	if existing == nil {
		saved, err := s.Create(model.Note{Title: fmTitle, Content: body, Tags: tags})
		if err != nil {
			return err
		}
		return git.CommitAndPush(repoPath, "note: add "+saved.ID)
	}
	existing.Title = fmTitle
	existing.Content = body
	existing.Tags = tags
	if err := s.Update(*existing); err != nil {
		return err
	}
	return git.CommitAndPush(repoPath, "note: update "+existing.ID)
}

func parseFrontmatter(text string) (title string, tags []string, content string) {
	if !strings.HasPrefix(text, "---\n") {
		return "", nil, text
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return "", nil, text
	}
	fm := rest[:end]
	content = strings.TrimPrefix(rest[end+4:], "\n")
	content = strings.TrimPrefix(content, "\n")
	for _, line := range strings.Split(fm, "\n") {
		if strings.HasPrefix(line, "title:") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
		} else if strings.HasPrefix(line, "tags:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "tags:"))
			for _, p := range strings.Split(val, ",") {
				if t := strings.TrimSpace(p); t != "" {
					tags = append(tags, t)
				}
			}
		}
	}
	return title, tags, content
}

func readPassphrase() (string, error) {
	fmt.Fprint(os.Stderr, "Passphrase: ")
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "note02: "+format+"\n", args...)
	os.Exit(1)
}
