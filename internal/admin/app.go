package admin

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"printrelay/internal/admin/api"
	"printrelay/internal/admin/ui"
	"printrelay/internal/admin/ui/common"
	"printrelay/internal/admin/ui/computers"
	"printrelay/internal/admin/ui/jobs"
	"printrelay/internal/admin/ui/printers"
	"printrelay/internal/admin/ui/tenants"
)

// Tab constants
const (
	TabComputers = iota
	TabPrinters
	TabJobs
	TabTenants
)

// Model is the root application model
type Model struct {
	config    Config
	client    *api.Client
	account   *api.Account
	activeTab int
	width     int
	height    int
	err       error

	// Child models
	computers computers.Model
	printers  printers.Model
	jobs      jobs.Model
	tenants   tenants.Model

	// State
	connected bool
	loading   bool

	// For gg command
	lastKey string
}

// New creates a new application model
func New(cfg Config) (Model, error) {
	client := api.NewClient(cfg.ServerURL, cfg.APIKey, cfg.AdminKey)

	return Model{
		config:    cfg,
		client:    client,
		activeTab: TabComputers,
		computers: computers.New(client),
		printers:  printers.New(client),
		jobs:      jobs.New(client),
		tenants:   tenants.New(client),
		loading:   true,
	}, nil
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.testConnection(),
		m.fetchAccount(),
		m.computers.Init(),
		m.printers.Init(),
		m.jobs.Init(),
	}
	if m.client.HasAdminAccess() {
		cmds = append(cmds, m.tenants.Init())
	}
	return tea.Batch(cmds...)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Handle gg for go to top
		if m.lastKey == "g" && key == "g" {
			m.lastKey = ""
			switch m.activeTab {
			case TabComputers:
				m.computers.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
				m.computers.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
			case TabPrinters:
				m.printers.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
				m.printers.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
			case TabJobs:
				m.jobs.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
				m.jobs.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
			case TabTenants:
				m.tenants.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
				m.tenants.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
			}
			return m, nil
		}
		m.lastKey = key

		// Global keys
		numTabs := 3
		if m.client.HasAdminAccess() {
			numTabs = 4
		}

		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.activeTab = TabComputers
			return m, nil
		case "2":
			m.activeTab = TabPrinters
			return m, nil
		case "3":
			m.activeTab = TabJobs
			return m, nil
		case "4":
			if m.client.HasAdminAccess() {
				m.activeTab = TabTenants
			}
			return m, nil
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % numTabs
			return m, nil
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab + numTabs - 1) % numTabs
			return m, nil
		case "R":
			return m, tea.Batch(
				m.fetchAccount(),
				func() tea.Msg { return common.RefreshMsg{} },
			)
		}

		// Delegate to active view
		var cmd tea.Cmd
		switch m.activeTab {
		case TabComputers:
			m.computers, cmd = m.computers.Update(msg)
		case TabPrinters:
			m.printers, cmd = m.printers.Update(msg)
		case TabJobs:
			m.jobs, cmd = m.jobs.Update(msg)
		case TabTenants:
			m.tenants, cmd = m.tenants.Update(msg)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := m.height - 8
		m.computers.SetSize(m.width-4, contentHeight)
		m.printers.SetSize(m.width-4, contentHeight)
		m.jobs.SetSize(m.width-4, contentHeight)
		m.tenants.SetSize(m.width-4, contentHeight)

	case connectionTestedMsg:
		m.loading = false
		if msg.err != nil {
			m.connected = false
			m.err = msg.err
		} else {
			m.connected = true
			m.err = nil
		}

	case common.AccountLoadedMsg:
		if msg.Err == nil {
			m.account = msg.Account
		}

	case common.ComputersLoadedMsg:
		m.computers, _ = m.computers.Update(msg)

	case common.PrintersLoadedMsg:
		m.printers, _ = m.printers.Update(msg)

	case common.JobsLoadedMsg:
		m.jobs, _ = m.jobs.Update(msg)

	case common.JobDeletedMsg:
		m.jobs, _ = m.jobs.Update(msg)

	case common.TenantsLoadedMsg:
		m.tenants, _ = m.tenants.Update(msg)

	case common.TenantCreatedMsg, common.TenantDeletedMsg, common.APIKeyAddedMsg, common.ClientKeyRotatedMsg:
		var cmd tea.Cmd
		m.tenants, cmd = m.tenants.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case common.RefreshMsg:
		var cmd tea.Cmd
		switch m.activeTab {
		case TabComputers:
			m.computers, cmd = m.computers.Update(msg)
		case TabPrinters:
			m.printers, cmd = m.printers.Update(msg)
		case TabJobs:
			m.jobs, cmd = m.jobs.Update(msg)
		case TabTenants:
			m.tenants, cmd = m.tenants.Update(msg)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the application
func (m Model) View() string {
	doc := strings.Builder{}

	// Header
	doc.WriteString(m.renderHeader())
	doc.WriteString("\n\n")

	// Tabs and content
	doc.WriteString(m.renderTabsAndContent())

	// Status bar
	doc.WriteString("\n")
	doc.WriteString(m.renderStatusBar())

	return doc.String()
}

func (m Model) renderHeader() string {
	title := ui.HeaderStyle.Render(" PrintRelay Admin ")

	serverInfo := m.config.ServerURL
	if m.account != nil && m.account.TenantName != "" {
		serverInfo = fmt.Sprintf("%s (%s)", m.config.ServerURL, m.account.TenantName)
	}

	var status string
	if m.loading {
		status = ui.MutedStyle.Render("[CONNECTING...]")
	} else if m.connected {
		status = ui.HeaderConnectedStyle.Render("[CONNECTED]")
	} else {
		status = ui.HeaderDisconnectedStyle.Render("[DISCONNECTED]")
	}

	return fmt.Sprintf("%s  %s  %s", title, serverInfo, status)
}

func (m Model) renderTabsAndContent() string {
	tabs := []struct {
		name  string
		count int
	}{
		{"Computers", m.computers.Count()},
		{"Printers", m.printers.Count()},
		{"Jobs", m.jobs.Count()},
	}

	if m.client.HasAdminAccess() {
		tabs = append(tabs, struct {
			name  string
			count int
		}{"Tenants", m.tenants.Count()})
	}

	// Calculate available width for tabs
	availableWidth := m.width - 2 // Account for borders

	// Calculate the natural width of each tab and total
	tabLabels := make([]string, len(tabs))
	naturalWidths := make([]int, len(tabs))
	totalNaturalWidth := 0

	for i, tab := range tabs {
		tabLabels[i] = fmt.Sprintf(" %d %s (%d) ", i+1, tab.name, tab.count)
		// Width includes border (2) and padding (2)
		naturalWidths[i] = len(tabLabels[i]) + 4
		totalNaturalWidth += naturalWidths[i]
	}

	// Calculate extra space to distribute to the last tab
	extraSpace := availableWidth - totalNaturalWidth
	if extraSpace < 0 {
		extraSpace = 0
	}

	var renderedTabs []string

	for i := range tabs {
		var style lipgloss.Style
		isFirst := i == 0
		isLast := i == len(tabs)-1
		isActive := i == m.activeTab

		label := tabLabels[i]

		// Add extra padding to the last tab to fill remaining space
		if isLast && extraSpace > 0 {
			label = label + strings.Repeat(" ", extraSpace)
		}

		if isActive {
			style = ui.TabActiveStyle
		} else {
			style = ui.TabInactiveStyle
		}

		// Adjust borders for connected tabs
		border, _, _, _, _ := style.GetBorder()
		if isFirst && isActive {
			border.BottomLeft = "│"
		} else if isFirst && !isActive {
			border.BottomLeft = "├"
		} else if isLast && isActive {
			border.BottomRight = "│"
		} else if isLast && !isActive {
			border.BottomRight = "┤"
		}
		style = style.Border(border)

		renderedTabs = append(renderedTabs, style.Render(label))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)

	// Content
	var content string
	switch m.activeTab {
	case TabComputers:
		content = m.computers.View()
	case TabPrinters:
		content = m.printers.View()
	case TabJobs:
		content = m.jobs.View()
	case TabTenants:
		content = m.tenants.View()
	}

	// Create the window with content - use full width
	windowWidth := m.width - 4 // Account for window borders
	if windowWidth < 40 {
		windowWidth = 40
	}

	window := ui.TabWindowStyle.Width(windowWidth).Render(content)

	return row + "\n" + window
}

func (m Model) renderStatusBar() string {
	var keys []string

	keys = append(keys,
		ui.StatusBarKeyStyle.Render("j/k")+" "+ui.StatusBarDescStyle.Render("navigate"),
		ui.StatusBarKeyStyle.Render("r")+" "+ui.StatusBarDescStyle.Render("refresh"),
	)

	if m.client.HasAdminAccess() {
		keys = append(keys, ui.StatusBarKeyStyle.Render("1-4")+" "+ui.StatusBarDescStyle.Render("tabs"))
	} else {
		keys = append(keys, ui.StatusBarKeyStyle.Render("1-3")+" "+ui.StatusBarDescStyle.Render("tabs"))
	}

	switch m.activeTab {
	case TabJobs:
		keys = append(keys, ui.StatusBarKeyStyle.Render("d")+" "+ui.StatusBarDescStyle.Render("delete"))
	case TabTenants:
		keys = append(keys, ui.StatusBarKeyStyle.Render("n")+" "+ui.StatusBarDescStyle.Render("new"))
		keys = append(keys, ui.StatusBarKeyStyle.Render("enter")+" "+ui.StatusBarDescStyle.Render("details"))
	}

	keys = append(keys, ui.StatusBarKeyStyle.Render("q")+" "+ui.StatusBarDescStyle.Render("quit"))

	content := strings.Join(keys, "  │  ")

	if m.err != nil {
		errMsg := ui.ErrorStyle.Render(" Error: " + m.err.Error())
		content = errMsg + "  │  " + content
	}

	return ui.StatusBarStyle.Width(m.width).Render(content)
}

type connectionTestedMsg struct {
	err error
}

func (m Model) testConnection() tea.Cmd {
	return func() tea.Msg {
		err := m.client.TestConnection()
		return connectionTestedMsg{err: err}
	}
}

func (m Model) fetchAccount() tea.Cmd {
	return func() tea.Msg {
		account, err := m.client.GetWhoami()
		return common.AccountLoadedMsg{Account: account, Err: err}
	}
}
