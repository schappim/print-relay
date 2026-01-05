package jobs

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"printrelay/internal/admin/api"
	"printrelay/internal/admin/ui"
	"printrelay/internal/admin/ui/common"
)

// Model is the jobs view model
type Model struct {
	client  *api.Client
	table   *common.Table
	jobs    []*api.PrintJob
	loading bool
	err     error
	width   int
	height  int
}

// New creates a new jobs view
func New(client *api.Client) Model {
	columns := []common.Column{
		{Title: "ID", Width: 8},
		{Title: "Title", Width: 24},
		{Title: "Printer", Width: 20},
		{Title: "State", Width: 14},
		{Title: "Created", Width: 14},
		{Title: "Source", Width: 10},
	}
	return Model{
		client:  client,
		table:   common.NewTable(columns),
		loading: true,
	}
}

// Init initializes the view
func (m Model) Init() tea.Cmd {
	return m.fetchJobs()
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
			return m, m.fetchJobs()
		case "d", "delete":
			if len(m.jobs) > 0 && m.table.SelectedRow() < len(m.jobs) {
				job := m.jobs[m.table.SelectedRow()]
				return m, m.deleteJob(job.ID)
			}
		}

	case common.JobsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.err = nil
		m.jobs = msg.Jobs
		m.updateTable()

	case common.JobDeletedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		// Refresh after delete
		m.loading = true
		return m, m.fetchJobs()

	case common.RefreshMsg:
		m.loading = true
		return m, m.fetchJobs()
	}

	return m, nil
}

// View renders the view
func (m Model) View() string {
	var sb strings.Builder

	if m.loading {
		sb.WriteString(ui.MutedStyle.Render("  Loading print jobs..."))
		sb.WriteString("\n")
	} else if m.err != nil {
		sb.WriteString(ui.ErrorStyle.Render("  Error: " + m.err.Error()))
		sb.WriteString("\n")
	} else {
		sb.WriteString(m.table.View())
	}

	return sb.String()
}

// Count returns the number of jobs
func (m Model) Count() int {
	return len(m.jobs)
}

// SetSize sets the view dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.Width = width
	m.table.Height = height - 4
}

func (m *Model) updateTable() {
	rows := make([][]string, len(m.jobs))
	for i, j := range m.jobs {
		stateStr := strings.ToUpper(j.State)

		// Format relative time
		created := formatRelativeTime(j.CreateTS)

		rows[i] = []string{
			fmt.Sprintf("%d", j.ID),
			j.Title,
			j.PrinterName,
			stateStr,
			created,
			j.Source,
		}

		// Set row styles
		styles := make([]lipgloss.Style, 6)
		for k := range styles {
			styles[k] = ui.TableCellStyle
		}
		styles[3] = ui.GetStateStyle(j.State) // State column
		m.table.SetRowStyles(i, styles)
	}
	m.table.SetRows(rows)
}

func (m Model) fetchJobs() tea.Cmd {
	return func() tea.Msg {
		jobs, err := m.client.GetPrintJobs()
		return common.JobsLoadedMsg{Jobs: jobs, Err: err}
	}
}

func (m Model) deleteJob(id int64) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeletePrintJob(id)
		return common.JobDeletedMsg{JobID: id, Err: err}
	}
}

func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	diff := time.Since(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
