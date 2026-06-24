package tui

import "github.com/charmbracelet/lipgloss"

// tagStyleFor returns a lipgloss style for a specific tag, using its configured
// color if present, otherwise falling back to the default styleTag.
func tagStyleFor(tag string, tagColors map[string]string) lipgloss.Style {
	if color, ok := tagColors[tag]; ok {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	}
	return styleTag
}

// Gruvbox dark palette
var (
	colorSelected  = lipgloss.Color("#fabd2f") // bright yellow
	colorMuted     = lipgloss.Color("#928374") // gray
	colorTag       = lipgloss.Color("#8ec07c") // bright aqua
	colorBorder    = lipgloss.Color("#504945") // bg2
	colorStatus    = lipgloss.Color("#fe8019") // bright orange
	colorErr       = lipgloss.Color("#fb4934") // bright red
	colorSynced    = lipgloss.Color("#b8bb26") // bright green
	colorSyncing   = lipgloss.Color("#fabd2f") // bright yellow
	colorConflict  = lipgloss.Color("#fb4934") // bright red
	colorTitle     = lipgloss.Color("#83a598") // bright blue
	colorPinned    = lipgloss.Color("220")     // gold (pinned rows)

	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(colorBorder)

	styleSelected = lipgloss.NewStyle().Bold(true).Foreground(colorSelected)
	styleMuted    = lipgloss.NewStyle().Foreground(colorMuted)
	styleTag      = lipgloss.NewStyle().Foreground(colorTag)
	stylePinned   = lipgloss.NewStyle().Foreground(colorPinned)
	styleStatus   = lipgloss.NewStyle().Foreground(colorStatus)
	styleErr      = lipgloss.NewStyle().Foreground(colorErr)

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)
)
