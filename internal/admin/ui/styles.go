package ui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	ColorPrimary      = lipgloss.Color("#7D56F4")
	ColorSecondary    = lipgloss.Color("#5A3FD9")
	ColorConnected    = lipgloss.Color("#00FF00")
	ColorDisconnected = lipgloss.Color("#FF5555")
	ColorPending      = lipgloss.Color("#FFFF00")
	ColorMuted        = lipgloss.Color("#666666")
	ColorBorder       = lipgloss.Color("#444444")
	ColorWhite        = lipgloss.Color("#FFFFFF")
	ColorBlack        = lipgloss.Color("#000000")
	ColorDarkGray     = lipgloss.Color("#333333")
)

// Header styles
var (
	HeaderStyle = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(ColorWhite).
			Bold(true).
			Padding(0, 1)

	HeaderConnectedStyle = lipgloss.NewStyle().
				Foreground(ColorConnected).
				Bold(true)

	HeaderDisconnectedStyle = lipgloss.NewStyle().
				Foreground(ColorDisconnected).
				Bold(true)
)

// Tab border helpers
func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

var (
	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")
)

// Tab styles
var (
	TabActiveStyle = lipgloss.NewStyle().
			Border(activeTabBorder, true).
			BorderForeground(ColorPrimary).
			Padding(0, 1)

	TabInactiveStyle = lipgloss.NewStyle().
				Border(inactiveTabBorder, true).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	TabWindowStyle = lipgloss.NewStyle().
			BorderForeground(ColorPrimary).
			Padding(1, 0).
			Border(lipgloss.NormalBorder()).
			UnsetBorderTop()
)

// Table styles
var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorWhite).
				Background(ColorSecondary).
				Padding(0, 1)

	TableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	TableSelectedStyle = lipgloss.NewStyle().
				Background(ColorPrimary).
				Foreground(ColorWhite).
				Bold(true).
				Padding(0, 1)

	TableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

// Status indicator styles
var (
	StatusConnected = lipgloss.NewStyle().
			Foreground(ColorConnected).
			Bold(true)

	StatusDisconnected = lipgloss.NewStyle().
				Foreground(ColorDisconnected)

	StatusPending = lipgloss.NewStyle().
			Foreground(ColorPending)

	StatusDone = lipgloss.NewStyle().
			Foreground(ColorConnected)

	StatusError = lipgloss.NewStyle().
			Foreground(ColorDisconnected).
			Bold(true)
)

// Status bar styles
var (
	StatusBarStyle = lipgloss.NewStyle().
			Background(ColorDarkGray).
			Foreground(ColorMuted).
			Padding(0, 1)

	StatusBarKeyStyle = lipgloss.NewStyle().
				Foreground(ColorWhite).
				Bold(true)

	StatusBarDescStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)
)

// General styles
var (
	BorderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorDisconnected).
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// GetStateStyle returns the appropriate style for a state string
func GetStateStyle(state string) lipgloss.Style {
	switch state {
	case "connected", "online", "done":
		return StatusConnected
	case "disconnected", "offline", "error", "cancelled":
		return StatusDisconnected
	case "new", "sent_to_client", "received", "downloading", "queued", "printing":
		return StatusPending
	default:
		return MutedStyle
	}
}
