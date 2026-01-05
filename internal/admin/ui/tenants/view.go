package tenants

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"printrelay/internal/admin/api"
	"printrelay/internal/admin/ui"
	"printrelay/internal/admin/ui/common"
)

// Mode represents the current view mode
type Mode int

const (
	ModeList Mode = iota
	ModeDetail
	ModeCreate
)

// Model is the tenants view model
type Model struct {
	client   *api.Client
	table    *common.Table
	tenants  []*api.Tenant
	loading  bool
	err      error
	width    int
	height   int
	mode     Mode
	selected *api.Tenant

	// For create mode
	inputName   string
	inputCursor int

	// For showing new keys
	showNewKey    bool
	newKeyType    string
	newKeyValue   string
}

// New creates a new tenants view
func New(client *api.Client) Model {
	columns := []common.Column{
		{Title: "ID", Width: 12},
		{Title: "Name", Width: 20},
		{Title: "API Keys", Width: 10},
		{Title: "State", Width: 10},
		{Title: "Created", Width: 20},
	}
	return Model{
		client:  client,
		table:   common.NewTable(columns),
		loading: true,
		mode:    ModeList,
	}
}

// Init initializes the view
func (m Model) Init() tea.Cmd {
	return m.fetchTenants()
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle create mode input
		if m.mode == ModeCreate {
			return m.handleCreateInput(msg)
		}

		// Handle detail mode
		if m.mode == ModeDetail {
			return m.handleDetailInput(msg)
		}

		// List mode
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
			return m, m.fetchTenants()
		case "enter":
			if len(m.tenants) > 0 && m.table.SelectedRow() < len(m.tenants) {
				m.selected = m.tenants[m.table.SelectedRow()]
				m.mode = ModeDetail
			}
		case "n", "c":
			m.mode = ModeCreate
			m.inputName = ""
			m.inputCursor = 0
		case "d", "delete":
			if len(m.tenants) > 0 && m.table.SelectedRow() < len(m.tenants) {
				tenant := m.tenants[m.table.SelectedRow()]
				return m, m.deleteTenant(tenant.ID)
			}
		}

	case common.TenantsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.err = nil
		m.tenants = msg.Tenants
		m.updateTable()

	case common.TenantCreatedMsg:
		m.mode = ModeList
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.loading = true
		return m, m.fetchTenants()

	case common.TenantDeletedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.loading = true
		return m, m.fetchTenants()

	case common.APIKeyAddedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.showNewKey = true
		m.newKeyType = "API Key"
		m.newKeyValue = msg.APIKey
		return m, m.fetchTenants()

	case common.ClientKeyRotatedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.showNewKey = true
		m.newKeyType = "Client Key"
		m.newKeyValue = msg.ClientKey
		return m, m.fetchTenants()

	case common.RefreshMsg:
		m.loading = true
		return m, m.fetchTenants()
	}

	return m, nil
}

func (m Model) handleCreateInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeList
	case "enter":
		if m.inputName != "" {
			return m, m.createTenant(m.inputName)
		}
	case "backspace":
		if len(m.inputName) > 0 {
			m.inputName = m.inputName[:len(m.inputName)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.inputName += msg.String()
		}
	}
	return m, nil
}

func (m Model) handleDetailInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = ModeList
		m.showNewKey = false
	case "a":
		// Add API key
		if m.selected != nil {
			return m, m.addAPIKey(m.selected.ID)
		}
	case "c":
		// Rotate client key
		if m.selected != nil {
			return m, m.rotateClientKey(m.selected.ID)
		}
	}
	return m, nil
}

// View renders the view
func (m Model) View() string {
	var sb strings.Builder

	switch m.mode {
	case ModeCreate:
		sb.WriteString(m.renderCreateForm())
	case ModeDetail:
		sb.WriteString(m.renderDetail())
	default:
		sb.WriteString(m.renderList())
	}

	return sb.String()
}

func (m Model) renderList() string {
	var sb strings.Builder

	if !m.client.HasAdminAccess() {
		sb.WriteString(ui.ErrorStyle.Render("  Admin key required to manage tenants"))
		sb.WriteString("\n")
		sb.WriteString(ui.MutedStyle.Render("  Use -admin-key flag or add adminKey to config"))
		return sb.String()
	}

	if m.loading {
		sb.WriteString(ui.MutedStyle.Render("  Loading tenants..."))
		sb.WriteString("\n")
	} else if m.err != nil {
		sb.WriteString(ui.ErrorStyle.Render("  Error: " + m.err.Error()))
		sb.WriteString("\n")
	} else {
		sb.WriteString(m.table.View())
	}

	return sb.String()
}

func (m Model) renderCreateForm() string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary)
	sb.WriteString(titleStyle.Render("  Create New Tenant"))
	sb.WriteString("\n\n")

	sb.WriteString("  Name: ")
	inputStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#333")).
		Foreground(lipgloss.Color("#fff")).
		Padding(0, 1)
	sb.WriteString(inputStyle.Render(m.inputName + "█"))
	sb.WriteString("\n\n")

	sb.WriteString(ui.MutedStyle.Render("  Enter: create | Esc: cancel"))

	return sb.String()
}

func (m Model) renderDetail() string {
	var sb strings.Builder

	if m.selected == nil {
		return "  No tenant selected"
	}

	t := m.selected
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0f0")).Background(lipgloss.Color("#222")).Padding(0, 1)

	sb.WriteString(titleStyle.Render("  Tenant: " + t.Name))
	sb.WriteString("\n\n")

	sb.WriteString("  " + labelStyle.Render("ID:") + valueStyle.Render(t.ID) + "\n")
	sb.WriteString("  " + labelStyle.Render("State:") + ui.GetStateStyle(t.State).Render(strings.ToUpper(t.State)) + "\n")
	sb.WriteString("  " + labelStyle.Render("Created:") + valueStyle.Render(t.CreatedAt.Format("2006-01-02 15:04")) + "\n")
	sb.WriteString("\n")

	sb.WriteString("  " + labelStyle.Render("Client Key:") + "\n")
	sb.WriteString("  " + keyStyle.Render(t.ClientKey) + "\n\n")

	sb.WriteString("  " + labelStyle.Render("Monitor Token:") + "\n")
	sb.WriteString("  " + keyStyle.Render(t.MonitorToken) + "\n\n")

	sb.WriteString("  " + labelStyle.Render("API Keys:") + fmt.Sprintf(" (%d)\n", len(t.APIKeys)))
	for i, key := range t.APIKeys {
		sb.WriteString(fmt.Sprintf("    %d. ", i+1) + keyStyle.Render(key) + "\n")
	}

	// Show new key if just generated
	if m.showNewKey {
		sb.WriteString("\n")
		newKeyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#0f0"))
		sb.WriteString("  " + newKeyStyle.Render("New "+m.newKeyType+":") + "\n")
		sb.WriteString("  " + keyStyle.Render(m.newKeyValue) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(ui.MutedStyle.Render("  a: add API key | c: rotate client key | Esc: back"))

	return sb.String()
}

// Count returns the number of tenants
func (m Model) Count() int {
	return len(m.tenants)
}

// SetSize sets the view dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.Width = width
	m.table.Height = height - 4
}

func (m *Model) updateTable() {
	rows := make([][]string, len(m.tenants))
	for i, t := range m.tenants {
		rows[i] = []string{
			t.ID,
			t.Name,
			fmt.Sprintf("%d", len(t.APIKeys)),
			strings.ToUpper(t.State),
			t.CreatedAt.Format("2006-01-02 15:04"),
		}

		styles := make([]lipgloss.Style, 5)
		for j := range styles {
			styles[j] = ui.TableCellStyle
		}
		styles[3] = ui.GetStateStyle(t.State)
		m.table.SetRowStyles(i, styles)
	}
	m.table.SetRows(rows)
}

func (m Model) fetchTenants() tea.Cmd {
	return func() tea.Msg {
		tenants, err := m.client.GetTenants()
		return common.TenantsLoadedMsg{Tenants: tenants, Err: err}
	}
}

func (m Model) createTenant(name string) tea.Cmd {
	return func() tea.Msg {
		tenant, err := m.client.CreateTenant(name)
		return common.TenantCreatedMsg{Tenant: tenant, Err: err}
	}
}

func (m Model) deleteTenant(id string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteTenant(id)
		return common.TenantDeletedMsg{TenantID: id, Err: err}
	}
}

func (m Model) addAPIKey(tenantID string) tea.Cmd {
	return func() tea.Msg {
		key, err := m.client.AddAPIKey(tenantID)
		return common.APIKeyAddedMsg{TenantID: tenantID, APIKey: key, Err: err}
	}
}

func (m Model) rotateClientKey(tenantID string) tea.Cmd {
	return func() tea.Msg {
		key, err := m.client.RotateClientKey(tenantID)
		return common.ClientKeyRotatedMsg{TenantID: tenantID, ClientKey: key, Err: err}
	}
}
