package tui

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/note02/internal/git"
	"github.com/yeniklas/note02/internal/model"
	"github.com/yeniklas/note02/internal/store"
)

type focusPanel int

const (
	panelList focusPanel = iota
	panelPreview
	panelSearch
	panelFilter
)

type syncState int

const (
	syncSynced   syncState = iota
	syncSyncing  syncState = iota
	syncConflict syncState = iota
)

type App struct {
	store       *store.Store
	list        listModel
	preview     previewModel
	search      searchModel
	filter      filterPopupModel
	focus       focusPanel
	notes       []model.Note
	filtered    []model.Note
	allTags     []string
	activeTag   string
	searchQuery string
	statusMsg   string
	errMsg      string
	deleteMode  bool // waiting for second 'd'
	syncState   syncState
	journalTags []string
	archiveTag  string
	width       int
	height      int
	markdown    bool

	// startup loading state
	loading      bool
	progress     progress.Model
	loadTotal    int
	loadDone     int
	loadingNotes []model.Note
}

func New(s *store.Store, markdown bool, journalTags []string, archiveTag string) *App {
	return &App{
		store:       s,
		list:        newListModel(),
		preview:     newPreviewModel(markdown),
		search:      newSearchModel(),
		filter:      newFilterPopupModel(),
		markdown:    markdown,
		journalTags: journalTags,
		archiveTag:  archiveTag,
		loading:     true,
		progress:    progress.New(progress.WithDefaultGradient()),
	}
}

func (a *App) Init() tea.Cmd {
	return a.pullCmd()
}

func (a *App) pullCmd() tea.Cmd {
	repoPath := a.store.RepoPath()
	return func() tea.Msg {
		return pullDoneMsg{err: git.Pull(repoPath)}
	}
}

// startLoadCmd enumerates note IDs (cheap, no decryption) so the total is known
// before per-note loading begins.
func (a *App) startLoadCmd() tea.Cmd {
	return func() tea.Msg {
		ids, err := a.store.ListIDs()
		if err != nil {
			return errMsg{err}
		}
		return loadStartMsg{ids}
	}
}

// loadNextCmd decrypts a single note by ID.
func (a *App) loadNextCmd(id string) tea.Cmd {
	return func() tea.Msg {
		note, err := a.store.Read(id)
		if err != nil {
			return errMsg{err}
		}
		return noteLoadedMsg{note}
	}
}

// finalizeLoadCmd sorts the accumulated notes and emits the terminal
// notesLoadedMsg, matching Store.List ordering (newest first).
func (a *App) finalizeLoadCmd() tea.Cmd {
	notes := a.loadingNotes
	model.SortNotes(notes)
	return func() tea.Msg {
		return notesLoadedMsg{notes}
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if w := msg.Width - 4; w < 60 {
			a.progress.Width = w
		} else {
			a.progress.Width = 60
		}
		a.relayout()
		return a, nil

	case pullDoneMsg:
		cmds := []tea.Cmd{a.startLoadCmd(), a.checkRepoStatusCmd()}
		if msg.err != nil {
			cmds = append(cmds, func() tea.Msg { return errMsg{msg.err} })
		}
		return a, tea.Batch(cmds...)

	case loadStartMsg:
		a.loadTotal = len(msg.ids)
		a.loadDone = 0
		a.loadingNotes = nil
		if a.loadTotal == 0 {
			return a, a.finalizeLoadCmd()
		}
		// Decrypt all notes concurrently: tea.Batch runs each command on its
		// own goroutine, so loads fan out across CPU cores instead of chaining
		// one after another.
		cmds := make([]tea.Cmd, len(msg.ids))
		for i, id := range msg.ids {
			cmds[i] = a.loadNextCmd(id)
		}
		return a, tea.Batch(cmds...)

	case noteLoadedMsg:
		a.loadingNotes = append(a.loadingNotes, msg.note)
		a.loadDone++
		if a.loadDone == a.loadTotal {
			return a, a.finalizeLoadCmd()
		}
		return a, nil

	case notesLoadedMsg:
		a.loading = false
		a.notes = msg.notes
		a.allTags = collectTags(a.notes)
		a.filter.setTags(a.allTags)
		a.applyFilter()
		a.list.setNotes(a.filtered)
		a.relayout()
		a.updatePreview()
		return a, nil

	case noteSavedMsg:
		a.notes = upsertNote(a.notes, msg.note)
		model.SortNotes(a.notes)
		a.allTags = collectTags(a.notes)
		a.filter.setTags(a.allTags)
		a.applyFilter()
		a.list.setNotes(a.filtered)
		a.relayout()
		for i, n := range a.filtered {
			if n.ID == msg.note.ID {
				a.list.cursor = i
				break
			}
		}
		a.updatePreview()
		a.statusMsg = "saved"
		a.syncState = syncSyncing
		return a, a.gitSyncCmd(msg.gitMsg)

	case noteDeletedMsg:
		a.notes = removeNote(a.notes, msg.id)
		a.allTags = collectTags(a.notes)
		a.filter.setTags(a.allTags)
		a.applyFilter()
		a.list.setNotes(a.filtered)
		a.relayout()
		a.updatePreview()
		a.statusMsg = "deleted"
		a.syncState = syncSyncing
		return a, a.gitSyncCmd(msg.gitMsg)

	case repoStatusMsg:
		if msg.err != nil {
			a.syncState = syncConflict
			a.errMsg = msg.err.Error()
		} else {
			a.syncState = msg.state
		}
		return a, nil

	case errMsg:
		a.errMsg = msg.err.Error()
		return a, nil

	case tea.KeyMsg:
		a.errMsg = ""
		return a.handleKey(msg)
	}

	if a.focus == panelSearch {
		var cmd tea.Cmd
		a.search, cmd = a.search.update(msg)
		a.searchQuery = a.search.value()
		a.applyFilter()
		a.list.setNotes(a.filtered)
		a.updatePreview()
		return a, cmd
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// quit always available
	if key.Matches(msg, keys.Quit) {
		return a, tea.Quit
	}

	switch a.focus {
	case panelSearch:
		return a.handleSearchKey(msg)
	case panelFilter:
		return a.handleFilterKey(msg)
	default:
		return a.handleListPreviewKey(msg)
	}
}

func (a *App) handleListPreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// preview scroll when focus is preview
	if a.focus == panelPreview {
		switch {
		case key.Matches(msg, keys.Up):
			a.preview.vp.LineUp(1)
			return a, nil
		case key.Matches(msg, keys.Down):
			a.preview.vp.LineDown(1)
			return a, nil
		case key.Matches(msg, keys.PageUp):
			a.preview.vp.HalfViewUp()
			return a, nil
		case key.Matches(msg, keys.PageDown):
			a.preview.vp.HalfViewDown()
			return a, nil
		case key.Matches(msg, keys.Tab):
			a.focus = panelList
			return a, nil
		}
	}

	switch {
	case key.Matches(msg, keys.Tab):
		if a.focus == panelList {
			a.focus = panelPreview
		} else {
			a.focus = panelList
		}
	case key.Matches(msg, keys.Up):
		a.deleteMode = false
		a.list.moveUp()
		a.updatePreview()
	case key.Matches(msg, keys.Down):
		a.deleteMode = false
		a.list.moveDown()
		a.updatePreview()
	case key.Matches(msg, keys.PageUp):
		a.deleteMode = false
		a.list.pageUp()
		a.updatePreview()
	case key.Matches(msg, keys.PageDown):
		a.deleteMode = false
		a.list.pageDown()
		a.updatePreview()
	case key.Matches(msg, keys.Top):
		a.list.jumpTop()
		a.updatePreview()
	case key.Matches(msg, keys.Bottom):
		a.list.jumpBottom()
		a.updatePreview()
	case key.Matches(msg, keys.New):
		a.deleteMode = false
		return a, a.openEditor(nil, "", nil)
	case key.Matches(msg, keys.Edit):
		a.deleteMode = false
		if note := a.list.selected(); note != nil {
			return a, a.openEditor(note, "", nil)
		}
	case key.Matches(msg, keys.Pin):
		a.deleteMode = false
		if note := a.list.selected(); note != nil {
			return a, a.togglePinCmd(*note)
		}
	case key.Matches(msg, keys.Archive):
		a.deleteMode = false
		if note := a.list.selected(); note != nil {
			return a, a.toggleArchiveCmd(*note)
		}
	case key.Matches(msg, keys.Journal):
		a.deleteMode = false
		return a, a.openJournalCmd()
	case key.Matches(msg, keys.Delete):
		if note := a.list.selected(); note != nil {
			if a.deleteMode {
				a.deleteMode = false
				return a, a.deleteNoteCmd(note.ID)
			}
			a.deleteMode = true
			a.statusMsg = "press d again to confirm delete"
		}
	case key.Matches(msg, keys.Search):
		a.deleteMode = false
		a.focus = panelSearch
		a.search.focus()
	case key.Matches(msg, keys.Filter):
		a.deleteMode = false
		a.focus = panelFilter
	case key.Matches(msg, keys.Clear):
		a.deleteMode = false
		a.activeTag = ""
		a.searchQuery = ""
		a.search.reset()
		a.applyFilter()
		a.list.setNotes(a.filtered)
		a.updatePreview()
		a.statusMsg = ""
	default:
		a.deleteMode = false
	}
	return a, nil
}

func (a *App) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		a.focus = panelList
		a.search.blur()
		if msg.String() == "esc" {
			a.searchQuery = ""
			a.search.reset()
			a.applyFilter()
			a.list.setNotes(a.filtered)
			a.updatePreview()
		}
		return a, nil
	}
	var cmd tea.Cmd
	a.search, cmd = a.search.update(msg)
	a.searchQuery = a.search.value()
	a.applyFilter()
	a.list.setNotes(a.filtered)
	a.updatePreview()
	return a, cmd
}

func (a *App) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.String() == "esc" || msg.String() == "q":
		a.focus = panelList
	case key.Matches(msg, keys.Up):
		a.filter.moveUp()
	case key.Matches(msg, keys.Down):
		a.filter.moveDown()
	case msg.String() == "enter":
		a.activeTag = a.filter.selected()
		a.applyFilter()
		a.list.setNotes(a.filtered)
		a.updatePreview()
		a.focus = panelList
		if a.activeTag != "" {
			a.statusMsg = "#" + a.activeTag
		}
	}
	return a, nil
}

func (a *App) openJournalCmd() tea.Cmd {
	title := "Journal " + time.Now().Format("2006-01-02")
	var existing *model.Note
	for i := range a.notes {
		if a.notes[i].Title == title {
			existing = &a.notes[i]
			break
		}
	}
	return a.openEditor(existing, title, a.journalTags)
}

func (a *App) openEditor(note *model.Note, defaultTitle string, defaultTags []string) tea.Cmd {
	title := defaultTitle
	tags := defaultTags
	content := ""
	if note != nil {
		title = note.Title
		tags = note.Tags
		content = note.Content
	}
	fileContent := writeFrontmatter(title, tags, content)

	tmp, err := os.CreateTemp("", "note02-*.md")
	if err != nil {
		a.errMsg = err.Error()
		return nil
	}
	if _, err := tmp.WriteString(fileContent); err != nil {
		tmp.Close()
		a.errMsg = err.Error()
		return nil
	}
	tmp.Close()

	hash := fileHash(tmp.Name())
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	return tea.ExecProcess(editorCmd(editor, tmp.Name()), func(err error) tea.Msg {
		if err != nil {
			os.Remove(tmp.Name())
			return errMsg{err}
		}
		newHash := fileHash(tmp.Name())
		data, readErr := os.ReadFile(tmp.Name())
		os.Remove(tmp.Name())
		if readErr != nil {
			return errMsg{readErr}
		}
		if newHash == hash && note != nil {
			return nil // no change
		}
		fmTitle, tags, body := parseFrontmatter(strings.TrimRight(string(data), "\n"))
		body = strings.TrimRight(body, "\n")
		if note == nil {
			saved, saveErr := a.store.Create(model.Note{
				Title:   fmTitle,
				Content: body,
				Tags:    tags,
			})
			if saveErr != nil {
				return errMsg{saveErr}
			}
			return noteSavedMsg{note: saved, gitMsg: "note: add " + saved.ID}
		}
		updated := *note
		updated.Title = fmTitle
		updated.Content = body
		updated.Tags = tags
		if saveErr := a.store.Update(updated); saveErr != nil {
			return errMsg{saveErr}
		}
		return noteSavedMsg{note: updated, gitMsg: "note: update " + updated.ID}
	})
}

// togglePinCmd adds or removes the pin tag on a note and persists it. Pinned
// notes sort to the top of the list (see model.SortNotes).
func (a *App) togglePinCmd(note model.Note) tea.Cmd {
	return func() tea.Msg {
		updated := note
		action := "pin"
		if note.IsPinned() {
			updated.Tags = removeTag(note.Tags, model.PinnedTag)
			action = "unpin"
		} else {
			updated.Tags = append(append([]string{}, note.Tags...), model.PinnedTag)
		}
		updated.UpdatedAt = time.Now().UTC()
		if err := a.store.Update(updated); err != nil {
			return errMsg{err}
		}
		return noteSavedMsg{note: updated, gitMsg: "note: " + action + " " + updated.ID}
	}
}

// toggleArchiveCmd adds or removes the archive tag on a note and persists it.
// Archived notes are hidden from the list (and search) unless the archive tag
// is the active filter (see applyFilter).
func (a *App) toggleArchiveCmd(note model.Note) tea.Cmd {
	tag := a.archiveTag
	return func() tea.Msg {
		updated := note
		action := "archive"
		if hasTag(note.Tags, tag) {
			updated.Tags = removeTag(note.Tags, tag)
			action = "unarchive"
		} else {
			updated.Tags = append(append([]string{}, note.Tags...), tag)
		}
		updated.UpdatedAt = time.Now().UTC()
		if err := a.store.Update(updated); err != nil {
			return errMsg{err}
		}
		return noteSavedMsg{note: updated, gitMsg: "note: " + action + " " + updated.ID}
	}
}

func removeTag(tags []string, target string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != target {
			out = append(out, t)
		}
	}
	return out
}

func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

func (a *App) deleteNoteCmd(id string) tea.Cmd {
	return func() tea.Msg {
		if err := a.store.Delete(id); err != nil {
			return errMsg{err}
		}
		return noteDeletedMsg{id: id, gitMsg: "note: delete " + id}
	}
}

func (a *App) gitSyncCmd(message string) tea.Cmd {
	repoPath := a.store.RepoPath()
	return func() tea.Msg {
		if err := git.CommitAndPush(repoPath, message); err != nil {
			status := git.CheckStatus(repoPath)
			state := syncConflict
			if status == git.RepoSynced {
				state = syncConflict // push failed but no conflict — still show error
			}
			return repoStatusMsg{state: state, err: err}
		}
		return repoStatusMsg{state: syncSynced}
	}
}

func (a *App) checkRepoStatusCmd() tea.Cmd {
	repoPath := a.store.RepoPath()
	return func() tea.Msg {
		status := git.CheckStatus(repoPath)
		if status == git.RepoConflict {
			return repoStatusMsg{state: syncConflict}
		}
		return repoStatusMsg{state: syncSynced}
	}
}

func (a *App) applyFilter() {
	filtered := make([]model.Note, 0, len(a.notes))
	query := strings.ToLower(strings.TrimSpace(a.searchQuery))

	for _, n := range a.notes {
		// Hide archived notes everywhere unless the archive tag is the active
		// filter (i.e. the user explicitly asked to see archived notes).
		if a.archiveTag != "" && a.activeTag != a.archiveTag && hasTag(n.Tags, a.archiveTag) {
			continue
		}
		if a.activeTag != "" {
			if !hasTag(n.Tags, a.activeTag) {
				continue
			}
		}
		if query != "" {
			haystack := strings.ToLower(n.Title + " " + n.Content)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		filtered = append(filtered, n)
	}
	a.filtered = filtered
}

func (a *App) updatePreview() {
	note := a.list.selected()
	a.preview.setNote(note)
}

func (a *App) relayout() {
	const minListW    = 24
	const minPreviewW = 40

	listW := a.list.preferredWidth()
	if max := a.width - minPreviewW - 1; listW > max {
		listW = max
	}
	if listW < minListW {
		listW = minListW
	}
	previewW := a.width - listW - 1

	bodyH := a.height - 3 // header + status bar + help bar
	a.list.setSize(listW, bodyH)
	a.preview.setSize(previewW, bodyH)
	a.filter.height = a.height / 2
}

func (a *App) View() string {
	if a.width == 0 {
		return "loading…"
	}

	if a.loading {
		return a.loadingView()
	}

	listW := a.list.width
	previewW := a.preview.width

	listContent := a.list.view(a.focus == panelList)
	previewContent := a.preview.view(a.focus == panelPreview)

	bodyH := a.height - 3
	listPane := stylePanel.Width(listW).Height(bodyH).Render(listContent)
	previewPane := lipgloss.NewStyle().Width(previewW).Height(bodyH).Render(previewContent)

	header := a.renderHeader()
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane)
	status := a.renderStatus()
	help := a.renderHelp()

	view := lipgloss.JoinVertical(lipgloss.Left, header, body, status, help)

	// overlays — render the popup centered on a clean canvas. Overlaying a
	// styled box onto the already-styled base by character offset corrupts the
	// ANSI escape sequences (scrambled output), so we replace the view instead,
	// the way memos-tui does.
	if a.focus == panelSearch {
		overlay := a.search.view(a.width)
		view = lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, overlay)
	} else if a.focus == panelFilter {
		overlay := a.filter.view(a.activeTag)
		view = lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, overlay)
	}

	return view
}

// loadingView renders a centered startup splash with a determinate progress bar.
func (a *App) loadingView() string {
	var percent float64
	if a.loadTotal > 0 {
		percent = float64(a.loadDone) / float64(a.loadTotal)
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(colorTitle).Render("note02")
	bar := a.progress.ViewAs(percent)
	count := styleMuted.Render(fmt.Sprintf("%d / %d notes", a.loadDone, a.loadTotal))

	content := lipgloss.JoinVertical(lipgloss.Center, title, "", bar, "", count)
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, content)
}

func (a *App) renderHeader() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(colorTitle).Render("note02")
	syncIndicator := a.renderSyncIndicator()
	gap := a.width - lipgloss.Width(title) - lipgloss.Width(syncIndicator)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + syncIndicator
}

func (a *App) renderSyncIndicator() string {
	switch a.syncState {
	case syncSyncing:
		return lipgloss.NewStyle().Foreground(colorSyncing).Render("● syncing…")
	case syncConflict:
		return lipgloss.NewStyle().Foreground(colorConflict).Render("● conflict")
	default:
		return lipgloss.NewStyle().Foreground(colorSynced).Render("● in sync")
	}
}

func (a *App) renderStatus() string {
	parts := []string{fmt.Sprintf("%d notes", len(a.filtered))}
	if a.activeTag != "" {
		parts = append(parts, styleTag.Render("#"+a.activeTag))
	}
	if a.searchQuery != "" {
		parts = append(parts, styleMuted.Render("search: "+a.searchQuery))
	}
	if a.errMsg != "" {
		parts = append(parts, styleErr.Render("error: "+a.errMsg))
	} else if a.statusMsg != "" {
		parts = append(parts, styleStatus.Render(a.statusMsg))
	}
	return strings.Join(parts, "  ·  ")
}

func (a *App) renderHelp() string {
	items := []string{
		"j/k:move", "tab:panel", "n:new", "e:edit", "p:pin", "a:archive", "d:delete",
		"J:journal", "/:search", "f:filter", "C:clear", "q:quit",
	}
	return styleMuted.Render(strings.Join(items, "  "))
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}


func collectTags(notes []model.Note) []string {
	seen := map[string]bool{}
	for _, n := range notes {
		for _, t := range n.Tags {
			if t == model.PinnedTag {
				continue
			}
			seen[t] = true
		}
	}
	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

func upsertNote(notes []model.Note, note model.Note) []model.Note {
	for i, n := range notes {
		if n.ID == note.ID {
			notes[i] = note
			return notes
		}
	}
	return append([]model.Note{note}, notes...)
}

func removeNote(notes []model.Note, id string) []model.Note {
	out := notes[:0]
	for _, n := range notes {
		if n.ID != id {
			out = append(out, n)
		}
	}
	return out
}
