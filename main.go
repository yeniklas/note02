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
	"github.com/yeniklas/note02/internal/crypto"
	"github.com/yeniklas/note02/internal/git"
	"github.com/yeniklas/note02/internal/model"
	"github.com/yeniklas/note02/internal/store"
	"github.com/yeniklas/note02/internal/tui"
	"github.com/yeniklas/note02/internal/updater"
	"golang.org/x/term"
)

var version = "dev"

func main() {
	journalFlag := flag.Bool("journal", false, "open or create today's journal entry and exit")
	versionFlag := flag.Bool("version", false, "print version and exit")
	updateFlag := flag.Bool("self-update", false, "update note02 to the latest release")
	changePassFlag := flag.Bool("change-passphrase", false, "re-encrypt the key under a new passphrase and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		return
	}

	if *updateFlag {
		if err := updater.Run(version); err != nil {
			fatalf("update: %v", err)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fatalf("config: %v", err)
	}
	if cfg.Repo.Path == "" {
		fatalf("repo.path is not set in ~/.config/note02/config.toml")
	}

	if *changePassFlag {
		if err := changePassphrase(cfg.Repo.Path); err != nil {
			fatalf("change passphrase: %v", err)
		}
		return
	}

	notesDir := filepath.Join(cfg.Repo.Path, "notes")
	if err := os.MkdirAll(notesDir, 0700); err != nil {
		fatalf("create notes dir: %v", err)
	}

	passphrase, err := readPassphrase()
	if err != nil {
		fatalf("passphrase: %v", err)
	}

	identity, err := crypto.LoadOrCreateIdentity(cfg.Repo.Path, passphrase)
	if err != nil {
		fatalf("identity: %v", err)
	}
	migrated, err := store.MigrateToIdentity(cfg.Repo.Path, passphrase, identity)
	if err != nil {
		fatalf("migrate: %v", err)
	}
	if migrated > 0 {
		if err := git.CommitAndPush(cfg.Repo.Path, fmt.Sprintf("note: migrate %d notes to identity encryption", migrated)); err != nil {
			fatalf("commit migration: %v", err)
		}
	}

	s := store.New(cfg.Repo.Path, identity)
	journalTags := cfg.Journal.EffectiveTags()

	if *journalFlag {
		if err := runJournal(s, cfg.Repo.Path, journalTags); err != nil {
			fatalf("journal: %v", err)
		}
		return
	}

	app := tui.New(s, cfg.Display.Markdown, journalTags, cfg.Archive.EffectiveTag())
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

	fmTitle, tags, fmCreatedAt, body := parseFrontmatter(strings.TrimRight(string(data), "\n"))
	body = strings.TrimRight(body, "\n")

	if existing == nil {
		saved, err := s.Create(model.Note{Title: fmTitle, Content: body, Tags: tags, CreatedAt: fmCreatedAt})
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

func parseFrontmatter(text string) (title string, tags []string, createdAt time.Time, content string) {
	if !strings.HasPrefix(text, "---\n") {
		return "", nil, time.Time{}, text
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return "", nil, time.Time{}, text
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
		} else if strings.HasPrefix(line, "created:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "created:"))
			if t, err := time.Parse("2006-01-02", val); err == nil {
				createdAt = t.UTC()
			}
		}
	}
	return title, tags, createdAt, content
}

// changePassphrase re-wraps the key file under a new passphrase. The X25519 key
// is unchanged, so notes are not re-encrypted; only identity.age is rewritten.
func changePassphrase(repoPath string) error {
	old, err := promptPassphrase("Current passphrase: ")
	if err != nil {
		return err
	}
	next, err := promptPassphrase("New passphrase: ")
	if err != nil {
		return err
	}
	confirm, err := promptPassphrase("Confirm new passphrase: ")
	if err != nil {
		return err
	}
	if next == "" {
		return fmt.Errorf("new passphrase must not be empty")
	}
	if next != confirm {
		return fmt.Errorf("passphrases do not match")
	}

	if err := crypto.ChangePassphrase(repoPath, old, next); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Passphrase changed.")
	if err := git.CommitAndPush(repoPath, "note: change passphrase"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not sync identity to git: %v\n", err)
	}
	return nil
}

func readPassphrase() (string, error) {
	return promptPassphrase("Passphrase: ")
}

func promptPassphrase(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
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
