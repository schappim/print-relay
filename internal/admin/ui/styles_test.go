package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestGetStateStyle(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  lipgloss.Style
	}{
		{
			name:  "connected",
			state: "connected",
			want:  StatusConnected,
		},
		{
			name:  "online",
			state: "online",
			want:  StatusConnected,
		},
		{
			name:  "done",
			state: "done",
			want:  StatusConnected,
		},
		{
			name:  "disconnected",
			state: "disconnected",
			want:  StatusDisconnected,
		},
		{
			name:  "offline",
			state: "offline",
			want:  StatusDisconnected,
		},
		{
			name:  "error",
			state: "error",
			want:  StatusDisconnected,
		},
		{
			name:  "cancelled",
			state: "cancelled",
			want:  StatusDisconnected,
		},
		{
			name:  "new",
			state: "new",
			want:  StatusPending,
		},
		{
			name:  "sent_to_client",
			state: "sent_to_client",
			want:  StatusPending,
		},
		{
			name:  "received",
			state: "received",
			want:  StatusPending,
		},
		{
			name:  "downloading",
			state: "downloading",
			want:  StatusPending,
		},
		{
			name:  "queued",
			state: "queued",
			want:  StatusPending,
		},
		{
			name:  "printing",
			state: "printing",
			want:  StatusPending,
		},
		{
			name:  "unknown state",
			state: "unknown",
			want:  MutedStyle,
		},
		{
			name:  "empty state",
			state: "",
			want:  MutedStyle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStateStyle(tt.state)
			// Compare styles by rendering a test string
			testStr := "test"
			if got.Render(testStr) != tt.want.Render(testStr) {
				t.Errorf("GetStateStyle(%q) rendered differently than expected", tt.state)
			}
		})
	}
}

func TestColors(t *testing.T) {
	// Test that colors are defined
	colors := []struct {
		name  string
		color lipgloss.Color
	}{
		{"ColorPrimary", ColorPrimary},
		{"ColorSecondary", ColorSecondary},
		{"ColorConnected", ColorConnected},
		{"ColorDisconnected", ColorDisconnected},
		{"ColorPending", ColorPending},
		{"ColorMuted", ColorMuted},
		{"ColorBorder", ColorBorder},
		{"ColorWhite", ColorWhite},
		{"ColorBlack", ColorBlack},
		{"ColorDarkGray", ColorDarkGray},
	}

	for _, c := range colors {
		t.Run(c.name, func(t *testing.T) {
			if c.color == "" {
				t.Errorf("%s is empty", c.name)
			}
		})
	}
}

func TestStyles(t *testing.T) {
	// Test that styles can render without panicking
	styles := []struct {
		name  string
		style lipgloss.Style
	}{
		{"HeaderStyle", HeaderStyle},
		{"HeaderConnectedStyle", HeaderConnectedStyle},
		{"HeaderDisconnectedStyle", HeaderDisconnectedStyle},
		{"TabActiveStyle", TabActiveStyle},
		{"TabInactiveStyle", TabInactiveStyle},
		{"TabWindowStyle", TabWindowStyle},
		{"TableHeaderStyle", TableHeaderStyle},
		{"TableRowStyle", TableRowStyle},
		{"TableSelectedStyle", TableSelectedStyle},
		{"TableCellStyle", TableCellStyle},
		{"StatusConnected", StatusConnected},
		{"StatusDisconnected", StatusDisconnected},
		{"StatusPending", StatusPending},
		{"StatusDone", StatusDone},
		{"StatusError", StatusError},
		{"StatusBarStyle", StatusBarStyle},
		{"StatusBarKeyStyle", StatusBarKeyStyle},
		{"StatusBarDescStyle", StatusBarDescStyle},
		{"BorderStyle", BorderStyle},
		{"TitleStyle", TitleStyle},
		{"ErrorStyle", ErrorStyle},
		{"MutedStyle", MutedStyle},
	}

	testStr := "Test String"
	for _, s := range styles {
		t.Run(s.name, func(t *testing.T) {
			// Should not panic
			result := s.style.Render(testStr)
			if result == "" {
				t.Errorf("%s.Render() returned empty string", s.name)
			}
		})
	}
}

func TestTabBorderWithBottom(t *testing.T) {
	border := tabBorderWithBottom("A", "B", "C")

	if border.BottomLeft != "A" {
		t.Errorf("BottomLeft = %q, want %q", border.BottomLeft, "A")
	}
	if border.Bottom != "B" {
		t.Errorf("Bottom = %q, want %q", border.Bottom, "B")
	}
	if border.BottomRight != "C" {
		t.Errorf("BottomRight = %q, want %q", border.BottomRight, "C")
	}

	// Should still have rounded top corners from base border
	if border.TopLeft != "╭" {
		t.Errorf("TopLeft = %q, want %q", border.TopLeft, "╭")
	}
	if border.TopRight != "╮" {
		t.Errorf("TopRight = %q, want %q", border.TopRight, "╮")
	}
}

func TestTabBorders(t *testing.T) {
	// Test that inactive tab border is configured correctly
	if inactiveTabBorder.BottomLeft != "┴" {
		t.Errorf("inactiveTabBorder.BottomLeft = %q, want %q", inactiveTabBorder.BottomLeft, "┴")
	}
	if inactiveTabBorder.Bottom != "─" {
		t.Errorf("inactiveTabBorder.Bottom = %q, want %q", inactiveTabBorder.Bottom, "─")
	}

	// Test that active tab border is configured correctly
	if activeTabBorder.BottomLeft != "┘" {
		t.Errorf("activeTabBorder.BottomLeft = %q, want %q", activeTabBorder.BottomLeft, "┘")
	}
	if activeTabBorder.Bottom != " " {
		t.Errorf("activeTabBorder.Bottom = %q, want %q", activeTabBorder.Bottom, " ")
	}
}
