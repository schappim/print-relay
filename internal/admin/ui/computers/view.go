package computers

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

// Model is the computers view model
type Model struct {
	client    *api.Client
	table     *common.Table
	computers []*protocol.Computer
	loading   bool
	err       error
	width     int
	height    int
}

// New creates a new computers view
func New(client *api.Client) Model {
	columns := []common.Column{
		{Title: "ID", Width: 6},
		{Title: "Name", Width: 20},
		{Title: "Hostname", Width: 20},
		{Title: "Version", Width: 10},
		{Title: "State", Width: 14},
		{Title: "IP", Width: 16},
	}
	return Model{
		client:  client,
		table:   common.NewTable(columns),
		loading: true,
	}
}

// Init initializes the view
func (m Model) Init() tea.Cmd {
	return m.fetchComputers()
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
			return m, m.fetchComputers()
		}

	case common.ComputersLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.err = nil
		m.computers = msg.Computers
		m.updateTable()

	case common.RefreshMsg:
		m.loading = true
		return m, m.fetchComputers()
	}

	return m, nil
}

// View renders the view
func (m Model) View() string {
	var sb strings.Builder

	if m.loading {
		sb.WriteString(ui.MutedStyle.Render("  Loading computers..."))
		sb.WriteString("\n")
	} else if m.err != nil {
		sb.WriteString(ui.ErrorStyle.Render("  Error: " + m.err.Error()))
		sb.WriteString("\n")
	} else {
		sb.WriteString(m.table.View())
	}

	return sb.String()
}

// Count returns the number of computers
func (m Model) Count() int {
	return len(m.computers)
}

// SetSize sets the view dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.Width = width
	m.table.Height = height - 4 // Account for header and status bar
}

func (m *Model) updateTable() {
	rows := make([][]string, len(m.computers))
	for i, c := range m.computers {
		ip := ""
		if c.Inet != nil {
			ip = *c.Inet
		}
		hostname := ""
		if c.Hostname != nil {
			hostname = *c.Hostname
		}
		version := ""
		if c.Version != nil {
			version = *c.Version
		}

		stateStr := strings.ToUpper(c.State)

		rows[i] = []string{
			fmt.Sprintf("%d", c.ID),
			c.Name,
			hostname,
			version,
			stateStr,
			ip,
		}

		// Set row styles based on state
		styles := make([]lipgloss.Style, 6)
		for j := range styles {
			styles[j] = ui.TableCellStyle
		}
		styles[4] = ui.GetStateStyle(c.State) // State column
		m.table.SetRowStyles(i, styles)
	}
	m.table.SetRows(rows)
}

func (m Model) fetchComputers() tea.Cmd {
	return func() tea.Msg {
		computers, err := m.client.GetComputers()
		return common.ComputersLoadedMsg{Computers: computers, Err: err}
	}
}
