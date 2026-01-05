package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"printrelay/internal/admin/ui"
)

// Column defines a table column
type Column struct {
	Title string
	Width int
}

// Table is a vim-navigable table component
type Table struct {
	Columns  []Column
	Rows     [][]string
	Styles   [][]lipgloss.Style // Optional per-cell styles
	Cursor   int
	Offset   int
	Height   int
	Width    int
	Focused  bool
}

// NewTable creates a new table
func NewTable(columns []Column) *Table {
	return &Table{
		Columns: columns,
		Rows:    make([][]string, 0),
		Styles:  make([][]lipgloss.Style, 0),
		Height:  10,
		Focused: true,
	}
}

// SetRows sets the table data
func (t *Table) SetRows(rows [][]string) {
	t.Rows = rows
	t.Styles = make([][]lipgloss.Style, len(rows))
	if t.Cursor >= len(rows) {
		t.Cursor = len(rows) - 1
	}
	if t.Cursor < 0 {
		t.Cursor = 0
	}
}

// SetRowStyles sets styles for a specific row
func (t *Table) SetRowStyles(rowIdx int, styles []lipgloss.Style) {
	if rowIdx >= 0 && rowIdx < len(t.Styles) {
		t.Styles[rowIdx] = styles
	}
}

// MoveUp moves the cursor up
func (t *Table) MoveUp() {
	if t.Cursor > 0 {
		t.Cursor--
		if t.Cursor < t.Offset {
			t.Offset = t.Cursor
		}
	}
}

// MoveDown moves the cursor down
func (t *Table) MoveDown() {
	if t.Cursor < len(t.Rows)-1 {
		t.Cursor++
		if t.Cursor >= t.Offset+t.Height {
			t.Offset = t.Cursor - t.Height + 1
		}
	}
}

// MoveToTop moves cursor to first row
func (t *Table) MoveToTop() {
	t.Cursor = 0
	t.Offset = 0
}

// MoveToBottom moves cursor to last row
func (t *Table) MoveToBottom() {
	if len(t.Rows) > 0 {
		t.Cursor = len(t.Rows) - 1
		if t.Cursor >= t.Height {
			t.Offset = t.Cursor - t.Height + 1
		}
	}
}

// SelectedRow returns the currently selected row index
func (t *Table) SelectedRow() int {
	return t.Cursor
}

// View renders the table
func (t *Table) View() string {
	var sb strings.Builder

	// Calculate total width and adjust column widths
	totalWidth := 0
	for _, col := range t.Columns {
		totalWidth += col.Width + 2 // +2 for padding
	}

	// Render header
	var headerCells []string
	for _, col := range t.Columns {
		cell := truncateOrPad(col.Title, col.Width)
		headerCells = append(headerCells, ui.TableHeaderStyle.Width(col.Width).Render(cell))
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, headerCells...))
	sb.WriteString("\n")

	// Render separator
	sb.WriteString(strings.Repeat("─", totalWidth))
	sb.WriteString("\n")

	// Render rows
	if len(t.Rows) == 0 {
		sb.WriteString(ui.MutedStyle.Render("  No data"))
		sb.WriteString("\n")
	} else {
		endIdx := t.Offset + t.Height
		if endIdx > len(t.Rows) {
			endIdx = len(t.Rows)
		}

		for i := t.Offset; i < endIdx; i++ {
			row := t.Rows[i]
			var rowStyles []lipgloss.Style
			if i < len(t.Styles) {
				rowStyles = t.Styles[i]
			}

			var cells []string
			for j, col := range t.Columns {
				cellValue := ""
				if j < len(row) {
					cellValue = row[j]
				}
				cellValue = truncateOrPad(cellValue, col.Width)

				var style lipgloss.Style
				if i == t.Cursor && t.Focused {
					style = ui.TableSelectedStyle.Width(col.Width)
				} else if j < len(rowStyles) {
					style = rowStyles[j].Width(col.Width).Padding(0, 1)
				} else {
					style = ui.TableCellStyle.Width(col.Width)
				}
				cells = append(cells, style.Render(cellValue))
			}

			// Add selection indicator
			prefix := "  "
			if i == t.Cursor && t.Focused {
				prefix = "▶ "
			}
			sb.WriteString(prefix)
			sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cells...))
			sb.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(t.Rows) > t.Height {
		scrollInfo := ui.MutedStyle.Render(
			strings.Repeat(" ", totalWidth-20) +
				formatScrollInfo(t.Offset, t.Height, len(t.Rows)),
		)
		sb.WriteString(scrollInfo)
	}

	return sb.String()
}

func truncateOrPad(s string, width int) string {
	if len(s) > width {
		if width > 3 {
			return s[:width-3] + "..."
		}
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func formatScrollInfo(offset, height, total int) string {
	if total <= height {
		return ""
	}
	start := offset + 1
	end := offset + height
	if end > total {
		end = total
	}
	return lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(
		strings.Repeat(" ", 5) + "[" + string(rune('0'+start/10)) + string(rune('0'+start%10)) + "-" +
			string(rune('0'+end/10)) + string(rune('0'+end%10)) + "/" +
			string(rune('0'+total/10)) + string(rune('0'+total%10)) + "]",
	)
}
