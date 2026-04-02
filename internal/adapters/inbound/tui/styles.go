package tui

import "github.com/charmbracelet/lipgloss"

// Palette — muted purple / teal theme.
var (
	colorBase    = lipgloss.Color("#1e1e2e")
	colorText    = lipgloss.Color("#cdd6f4")
	colorSubtle  = lipgloss.Color("#6c7086")
	colorPrimary = lipgloss.Color("#cba6f7")
	colorTeal    = lipgloss.Color("#89dceb")
	colorGreen   = lipgloss.Color("#a6e3a1")
	colorRed     = lipgloss.Color("#f38ba8")
	colorGold    = lipgloss.Color("#f9e2af")
	colorBlue    = lipgloss.Color("#89b4fa")
	colorOverlay = lipgloss.Color("#313244")
)

var (
	styleApp = lipgloss.NewStyle().
			Foreground(colorText)

	styleLogo = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText)

	styleSubtle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	styleSelected = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleCursor = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true)

	styleTag = lipgloss.NewStyle().
			Foreground(colorBlue).
			Padding(0, 1)

	styleID = lipgloss.NewStyle().
		Foreground(colorSubtle)

	styleProject = lipgloss.NewStyle().
			Foreground(colorGold)

	styleError = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Italic(true)

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorOverlay).
			Padding(0, 1)

	styleCount = lipgloss.NewStyle().
			Foreground(colorTeal)
)
