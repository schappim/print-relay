package printers

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"printrelay/internal/admin/api"
	"printrelay/internal/admin/ui"
	"printrelay/internal/admin/ui/common"
	"printrelay/protocol"
)

// Model is the printers view model
type Model struct {
	client   *api.Client
	table    *common.Table
	printers []*protocol.Printer
	loading  bool
	err      error
	width    int
	height   int
}

// New creates a new printers view
func New(client *api.Client) Model {
	columns := []common.Column{
		{Title: "ID", Width: 6},
		{Title: "Name", Width: 28},
		{Title: "Computer", Width: 12},
		{Title: "Default", Width: 8},
		{Title: "State", Width: 10},
		{Title: "Capabilities", Width: 20},
	}
	return Model{
		client:  client,
		table:   common.NewTable(columns),
		loading: true,
	}
}

// Init initializes the view
func (m Model) Init() tea.Cmd {
	return m.fetchPrinters()
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.table.MoveDown()
		case "k", "up":
			m.table.MoveUp()
		case "g":
			m.table.MoveToTop()
		case "G":
			m.table.MoveToBottom()
		case "r":
			m.loading = true
			return m, m.fetchPrinters()
		}

	case common.PrintersLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.err = nil
		m.printers = msg.Printers
		m.updateTable()

	case common.RefreshMsg:
		m.loading = true
		return m, m.fetchPrinters()
	}

	return m, nil
}

// View renders the view
func (m Model) View() string {
	var sb strings.Builder

	if m.loading {
		sb.WriteString(ui.MutedStyle.Render("  Loading printers..."))
		sb.WriteString("\n")
	} else if m.err != nil {
		sb.WriteString(ui.ErrorStyle.Render("  Error: " + m.err.Error()))
		sb.WriteString("\n")
	} else {
		sb.WriteString(m.table.View())
	}

	return sb.String()
}

// Count returns the number of printers
func (m Model) Count() int {
	return len(m.printers)
}

// SetSize sets the view dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.Width = width
	m.table.Height = height - 4
}

func (m *Model) updateTable() {
	rows := make([][]string, len(m.printers))
	for i, p := range m.printers {
		defaultStr := ""
		if p.Default {
			defaultStr = "Yes"
		}

		// Build capabilities string
		caps := []string{}
		if p.Capabilities.Color {
			caps = append(caps, "Color")
		}
		if p.Capabilities.Duplex {
			caps = append(caps, "Duplex")
		}
		if p.Capabilities.Collate {
			caps = append(caps, "Collate")
		}
		capsStr := strings.Join(caps, ", ")
		if capsStr == "" {
			capsStr = "-"
		}

		stateStr := strings.ToUpper(p.State)
		if stateStr == "" {
			stateStr = "UNKNOWN"
		}

		rows[i] = []string{
			fmt.Sprintf("%d", p.ID),
			p.Name,
			fmt.Sprintf("%d", p.ComputerID),
			defaultStr,
			stateStr,
			capsStr,
		}

		// Set row styles
		styles := make([]lipgloss.Style, 6)
		for j := range styles {
			styles[j] = ui.TableCellStyle
		}
		styles[4] = ui.GetStateStyle(p.State) // State column
		m.table.SetRowStyles(i, styles)
	}
	m.table.SetRows(rows)
}

func (m Model) fetchPrinters() tea.Cmd {
	return func() tea.Msg {
		printers, err := m.client.GetPrinters()
		return common.PrintersLoadedMsg{Printers: printers, Err: err}
	}
}
