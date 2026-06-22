package tui

import "github.com/charmbracelet/lipgloss"

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

	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(colorBorder)

	styleSelected = lipgloss.NewStyle().Bold(true).Foreground(colorSelected)
	styleMuted    = lipgloss.NewStyle().Foreground(colorMuted)
	styleTag      = lipgloss.NewStyle().Foreground(colorTag)
	styleStatus   = lipgloss.NewStyle().Foreground(colorStatus)
	styleErr      = lipgloss.NewStyle().Foreground(colorErr)

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)
)
