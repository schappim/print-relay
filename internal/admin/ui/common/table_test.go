package common

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewTable(t *testing.T) {
	columns := []Column{
		{Title: "ID", Width: 10},
		{Title: "Name", Width: 20},
	}

	table := NewTable(columns)

	if table == nil {
		t.Fatal("NewTable() returned nil")
	}
	if len(table.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(table.Columns))
	}
	if table.Cursor != 0 {
		t.Errorf("Initial cursor = %d, want 0", table.Cursor)
	}
	if table.Offset != 0 {
		t.Errorf("Initial offset = %d, want 0", table.Offset)
	}
	if table.Height != 10 {
		t.Errorf("Default height = %d, want 10", table.Height)
	}
	if !table.Focused {
		t.Error("Table should be focused by default")
	}
}

func TestTable_SetRows(t *testing.T) {
	columns := []Column{
		{Title: "ID", Width: 10},
		{Title: "Name", Width: 20},
	}
	table := NewTable(columns)

	rows := [][]string{
		{"1", "Row 1"},
		{"2", "Row 2"},
		{"3", "Row 3"},
	}
	table.SetRows(rows)

	if len(table.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(table.Rows))
	}
	if len(table.Styles) != 3 {
		t.Errorf("Expected 3 style rows, got %d", len(table.Styles))
	}
}

func TestTable_SetRows_ResetsCursor(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})

	// Set initial rows and move cursor
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}})
	table.Cursor = 2

	// Set fewer rows - cursor should adjust
	table.SetRows([][]string{{"1"}})
	if table.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0 after reducing rows", table.Cursor)
	}
}

func TestTable_SetRows_Empty(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{})

	if table.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0 for empty rows", table.Cursor)
	}
}

func TestTable_SetRowStyles(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{{"1"}, {"2"}})

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	table.SetRowStyles(0, []lipgloss.Style{style})

	if len(table.Styles[0]) != 1 {
		t.Errorf("Expected 1 style, got %d", len(table.Styles[0]))
	}
}

func TestTable_SetRowStyles_OutOfBounds(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{{"1"}})

	// Should not panic
	table.SetRowStyles(-1, []lipgloss.Style{})
	table.SetRowStyles(100, []lipgloss.Style{})
}

func TestTable_MoveUp(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}})

	// Start at 0, move up should stay at 0
	table.MoveUp()
	if table.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", table.Cursor)
	}

	// Set cursor to 2, move up should go to 1
	table.Cursor = 2
	table.MoveUp()
	if table.Cursor != 1 {
		t.Errorf("Cursor = %d, want 1", table.Cursor)
	}
}

func TestTable_MoveDown(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}})

	// Start at 0, move down should go to 1
	table.MoveDown()
	if table.Cursor != 1 {
		t.Errorf("Cursor = %d, want 1", table.Cursor)
	}

	// Move to last row
	table.Cursor = 2
	table.MoveDown()
	if table.Cursor != 2 {
		t.Errorf("Cursor = %d, want 2 (should not go past last row)", table.Cursor)
	}
}

func TestTable_MoveUp_AdjustsOffset(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.Height = 2
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}})

	// Set cursor and offset to middle
	table.Cursor = 2
	table.Offset = 2

	// Move up should adjust offset
	table.MoveUp()
	if table.Cursor != 1 {
		t.Errorf("Cursor = %d, want 1", table.Cursor)
	}
	if table.Offset != 1 {
		t.Errorf("Offset = %d, want 1", table.Offset)
	}
}

func TestTable_MoveDown_AdjustsOffset(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.Height = 2
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}})

	// Move down past visible area
	table.MoveDown() // 0 -> 1
	table.MoveDown() // 1 -> 2, should adjust offset

	if table.Cursor != 2 {
		t.Errorf("Cursor = %d, want 2", table.Cursor)
	}
	if table.Offset != 1 {
		t.Errorf("Offset = %d, want 1", table.Offset)
	}
}

func TestTable_MoveToTop(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}})
	table.Cursor = 2
	table.Offset = 1

	table.MoveToTop()

	if table.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", table.Cursor)
	}
	if table.Offset != 0 {
		t.Errorf("Offset = %d, want 0", table.Offset)
	}
}

func TestTable_MoveToBottom(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.Height = 2
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}})

	table.MoveToBottom()

	if table.Cursor != 4 {
		t.Errorf("Cursor = %d, want 4", table.Cursor)
	}
	if table.Offset != 3 {
		t.Errorf("Offset = %d, want 3", table.Offset)
	}
}

func TestTable_MoveToBottom_EmptyTable(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{})

	table.MoveToBottom()

	if table.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0 for empty table", table.Cursor)
	}
}

func TestTable_SelectedRow(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}})
	table.Cursor = 1

	if table.SelectedRow() != 1 {
		t.Errorf("SelectedRow() = %d, want 1", table.SelectedRow())
	}
}

func TestTable_View(t *testing.T) {
	columns := []Column{
		{Title: "ID", Width: 10},
		{Title: "Name", Width: 20},
	}
	table := NewTable(columns)
	table.SetRows([][]string{
		{"1", "Row 1"},
		{"2", "Row 2"},
	})

	view := table.View()

	// Check header is rendered
	if !strings.Contains(view, "ID") {
		t.Error("View should contain header 'ID'")
	}
	if !strings.Contains(view, "Name") {
		t.Error("View should contain header 'Name'")
	}

	// Check separator
	if !strings.Contains(view, "─") {
		t.Error("View should contain separator")
	}

	// Check selection indicator
	if !strings.Contains(view, "▶") {
		t.Error("View should contain selection indicator")
	}
}

func TestTable_View_EmptyTable(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{})

	view := table.View()

	if !strings.Contains(view, "No data") {
		t.Error("Empty table should show 'No data' message")
	}
}

func TestTable_View_Unfocused(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.SetRows([][]string{{"1"}, {"2"}})
	table.Focused = false

	view := table.View()

	// Selection indicator should not be present when unfocused
	if strings.Contains(view, "▶") {
		t.Error("Unfocused table should not show selection indicator")
	}
}

func TestTable_View_WithScroll(t *testing.T) {
	table := NewTable([]Column{{Title: "ID", Width: 10}})
	table.Height = 2
	table.SetRows([][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}})

	view := table.View()

	// Should show scroll indicator when there are more rows than visible
	if !strings.Contains(view, "/") {
		t.Error("Table with scroll should show scroll indicator")
	}
}

func TestTruncateOrPad(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "exact length",
			input: "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "needs padding",
			input: "hi",
			width: 5,
			want:  "hi   ",
		},
		{
			name:  "needs truncation",
			input: "hello world",
			width: 8,
			want:  "hello...",
		},
		{
			name:  "very short width",
			input: "hello",
			width: 2,
			want:  "he",
		},
		{
			name:  "empty string",
			input: "",
			width: 5,
			want:  "     ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateOrPad(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("truncateOrPad(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

func TestFormatScrollInfo(t *testing.T) {
	tests := []struct {
		name   string
		offset int
		height int
		total  int
		empty  bool
	}{
		{
			name:   "no scroll needed",
			offset: 0,
			height: 10,
			total:  5,
			empty:  true,
		},
		{
			name:   "scroll at top",
			offset: 0,
			height: 5,
			total:  20,
			empty:  false,
		},
		{
			name:   "scroll in middle",
			offset: 5,
			height: 5,
			total:  20,
			empty:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatScrollInfo(tt.offset, tt.height, tt.total)
			if tt.empty && got != "" {
				t.Errorf("formatScrollInfo() = %q, want empty", got)
			}
			if !tt.empty && got == "" {
				t.Error("formatScrollInfo() returned empty, want non-empty")
			}
		})
	}
}
