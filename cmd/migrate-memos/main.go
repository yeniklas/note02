// Command migrate-memos is a one-time tool that imports all memos from a
// self-hosted Memos instance (https://github.com/usememos/memos) into note02.
//
// It reads server credentials from the memos-tui config
// (~/.config/memos-tui/config.toml) unless -url and -token are supplied,
// fetches every memo (active and archived) via the Memos REST API, maps each
// to a note02 note, encrypts it under the note02 passphrase, and writes it into
// the configured notes repo with a single git commit.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/term"

	"github.com/yeniklas/note02/internal/config"
	"github.com/yeniklas/note02/internal/crypto"
	"github.com/yeniklas/note02/internal/git"
	"github.com/yeniklas/note02/internal/model"
	"github.com/yeniklas/note02/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		profile  = flag.String("profile", "", "memos-tui profile name (default: config's default_profile)")
		urlFlag  = flag.String("url", "", "Memos server URL (overrides config)")
		token    = flag.String("token", "", "Memos access token (overrides config)")
		dryRun   = flag.Bool("dry-run", false, "print what would be imported without writing")
		noCommit = flag.Bool("no-commit", false, "skip the final git commit/push")
	)
	flag.Parse()

	// 1. Resolve Memos credentials.
	baseURL, tok := *urlFlag, *token
	if baseURL == "" || tok == "" {
		p, err := loadMemosProfile(*profile)
		if err != nil {
			return err
		}
		if baseURL == "" {
			baseURL = p.URL
		}
		if tok == "" {
			tok = p.Token
		}
	}
	if baseURL == "" || tok == "" {
		return fmt.Errorf("missing Memos url/token (set them in memos-tui config or via -url/-token)")
	}

	// 2. Load note02 config + passphrase.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load note02 config: %w", err)
	}
	if cfg.Repo.Path == "" {
		return fmt.Errorf("note02 config has no repo.path set")
	}

	// 3. Fetch all memos (active + archived).
	client := &memosClient{baseURL: strings.TrimRight(baseURL, "/"), token: tok, http: &http.Client{}}
	fmt.Fprintf(os.Stderr, "Fetching memos from %s ...\n", client.baseURL)
	active, err := client.listAll(false)
	if err != nil {
		return fmt.Errorf("list active memos: %w", err)
	}
	archived, err := client.listAll(true)
	if err != nil {
		return fmt.Errorf("list archived memos: %w", err)
	}
	memos := append(active, archived...)
	fmt.Fprintf(os.Stderr, "Found %d memos (%d active, %d archived).\n", len(memos), len(active), len(archived))

	// 4. Map memos -> notes.
	notes := make([]model.Note, len(memos))
	for i, m := range memos {
		notes[i] = toNote(m)
	}

	if *dryRun {
		for _, n := range notes {
			fmt.Printf("%s  %-50s  [%s]\n", n.CreatedAt.Format("2006-01-02"), truncate(n.Title, 50), strings.Join(n.Tags, ", "))
		}
		fmt.Fprintf(os.Stderr, "\nDry run: %d notes would be imported. Nothing written.\n", len(notes))
		return nil
	}

	if len(notes) == 0 {
		fmt.Fprintln(os.Stderr, "No memos to import.")
		return nil
	}

	// 5. Passphrase + verify against existing repo.
	passphrase, err := readPassphrase()
	if err != nil {
		return fmt.Errorf("read passphrase: %w", err)
	}
	identity, err := crypto.LoadOrCreateIdentity(cfg.Repo.Path, passphrase)
	if err != nil {
		return fmt.Errorf("unlock identity (wrong passphrase?): %w", err)
	}
	if _, err := store.MigrateToIdentity(cfg.Repo.Path, passphrase, identity); err != nil {
		return fmt.Errorf("migrate existing notes: %w", err)
	}
	st := store.New(cfg.Repo.Path, identity)
	if _, err := st.List(); err != nil {
		return fmt.Errorf("verify against existing notes: %w", err)
	}

	// Ensure notes dir exists before writing.
	if err := os.MkdirAll(filepath.Join(cfg.Repo.Path, "notes"), 0700); err != nil {
		return fmt.Errorf("create notes dir: %w", err)
	}

	// 6. Write notes.
	for _, n := range notes {
		if _, err := st.Import(n); err != nil {
			return fmt.Errorf("import note %q: %w", n.Title, err)
		}
	}
	fmt.Fprintf(os.Stderr, "Imported %d notes into %s/notes\n", len(notes), cfg.Repo.Path)

	// 7. Single git commit.
	if !*noCommit {
		msg := fmt.Sprintf("import: migrate %d memos from %s", len(notes), client.baseURL)
		if err := git.CommitAndPush(cfg.Repo.Path, msg); err != nil {
			return fmt.Errorf("git commit: %w", err)
		}
		fmt.Fprintln(os.Stderr, "Committed.")
	}
	return nil
}

// --- memos-tui config ---

type memosProfile struct {
	URL   string `toml:"url"`
	Token string `toml:"token"`
}

type memosConfig struct {
	DefaultProfile string                  `toml:"default_profile"`
	Profiles       map[string]memosProfile `toml:"profiles"`
}

func loadMemosProfile(name string) (memosProfile, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return memosProfile{}, err
	}
	path := filepath.Join(dir, "memos-tui", "config.toml")
	var cfg memosConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return memosProfile{}, fmt.Errorf("read memos-tui config %s: %w", path, err)
	}
	if name == "" {
		name = cfg.DefaultProfile
	}
	if name == "" {
		return memosProfile{}, fmt.Errorf("no profile given and no default_profile in %s", path)
	}
	p, ok := cfg.Profiles[name]
	if !ok {
		return memosProfile{}, fmt.Errorf("profile %q not found in %s", name, path)
	}
	return p, nil
}

// --- minimal Memos REST client ---

type memo struct {
	Content    string    `json:"content"`
	Pinned     bool      `json:"pinned"`
	State      string    `json:"state"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
	Tags       []string  `json:"tags"`
}

type listMemosResponse struct {
	Memos         []memo `json:"memos"`
	NextPageToken string `json:"nextPageToken"`
}

type memosClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func (c *memosClient) listAll(archived bool) ([]memo, error) {
	var all []memo
	pageToken := ""
	for {
		params := url.Values{"pageSize": {"50"}}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		if archived {
			params.Set("state", "ARCHIVED")
		}
		req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v1/memos?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			resp.Body.Close()
			return nil, fmt.Errorf("API error %d", resp.StatusCode)
		}
		var r listMemosResponse
		err = json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		all = append(all, r.Memos...)
		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
	return all, nil
}

// --- mapping ---

var tagRE = regexp.MustCompile(`#(\w+(?:/\w+)*)`)

func toNote(m memo) model.Note {
	tags := m.Tags
	if len(tags) == 0 {
		tags = extractTags(m.Content)
	}
	seen := map[string]bool{}
	deduped := make([]string, 0, len(tags)+2)
	add := func(t string) {
		if t != "" && !seen[t] {
			seen[t] = true
			deduped = append(deduped, t)
		}
	}
	for _, t := range tags {
		add(t)
	}
	if m.Pinned {
		add("pinned")
	}
	if m.State == "ARCHIVED" {
		add("archived")
	}

	return model.Note{
		Title:     deriveTitle(m.Content),
		Content:   m.Content,
		Tags:      deduped,
		CreatedAt: m.CreateTime,
		UpdatedAt: m.UpdateTime,
	}
}

func extractTags(content string) []string {
	matches := tagRE.FindAllStringSubmatch(content, -1)
	seen := map[string]bool{}
	var tags []string
	for _, mm := range matches {
		t := mm[1]
		if !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}
	return tags
}

func deriveTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#")
		line = strings.TrimSpace(line)
		if line != "" {
			return truncate(line, 80)
		}
	}
	return "(untitled)"
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func readPassphrase() (string, error) {
	fmt.Fprint(os.Stderr, "note02 passphrase: ")
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
