package admin

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"printrelay/internal/admin/api"
	"printrelay/internal/admin/ui/common"
	"printrelay/protocol"
)

func TestNew(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}

	model, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if model.client == nil {
		t.Error("client should not be nil")
	}
	if model.activeTab != TabComputers {
		t.Errorf("activeTab = %d, want TabComputers", model.activeTab)
	}
	if !model.loading {
		t.Error("model should start in loading state")
	}
}

func TestNew_WithAdminKey(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
		AdminKey:  "admin-key",
	}

	model, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if !model.client.HasAdminAccess() {
		t.Error("client should have admin access")
	}
}

func TestModel_Init(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}

	model, _ := New(cfg)
	cmd := model.Init()

	if cmd == nil {
		t.Error("Init() should return a command")
	}
}

func TestModel_Update_TabSwitching(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	// Test switching to tab 1 (Computers)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m := updated.(Model)
	if m.activeTab != TabComputers {
		t.Errorf("activeTab = %d, want TabComputers", m.activeTab)
	}

	// Test switching to tab 2 (Printers)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(Model)
	if m.activeTab != TabPrinters {
		t.Errorf("activeTab = %d, want TabPrinters", m.activeTab)
	}

	// Test switching to tab 3 (Jobs)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = updated.(Model)
	if m.activeTab != TabJobs {
		t.Errorf("activeTab = %d, want TabJobs", m.activeTab)
	}
}

func TestModel_Update_TabSwitching_WithAdminAccess(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
		AdminKey:  "admin-key",
	}
	model, _ := New(cfg)

	// Test switching to tab 4 (Tenants) - only available with admin access
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m := updated.(Model)
	if m.activeTab != TabTenants {
		t.Errorf("activeTab = %d, want TabTenants", m.activeTab)
	}
}

func TestModel_Update_TabCycling(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	// Test Tab key cycles forward
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	m := updated.(Model)
	if m.activeTab != TabPrinters {
		t.Errorf("activeTab = %d, want TabPrinters", m.activeTab)
	}

	// Test Shift+Tab cycles backward
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.activeTab != TabComputers {
		t.Errorf("activeTab = %d, want TabComputers", m.activeTab)
	}

	// Test 'l' key cycles forward
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	if m.activeTab != TabPrinters {
		t.Errorf("activeTab = %d, want TabPrinters after 'l'", m.activeTab)
	}

	// Test 'h' key cycles backward
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(Model)
	if m.activeTab != TabComputers {
		t.Errorf("activeTab = %d, want TabComputers after 'h'", m.activeTab)
	}
}

func TestModel_Update_Quit(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	// Test 'q' quits
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("'q' should return a quit command")
	}

	// Test Ctrl+C quits
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Ctrl+C should return a quit command")
	}
}

func TestModel_Update_WindowSizeMsg(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updated.(Model)

	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
}

func TestModel_Update_ConnectionTestedMsg(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	// Test successful connection
	updated, _ := model.Update(connectionTestedMsg{err: nil})
	m := updated.(Model)

	if m.loading {
		t.Error("loading should be false after connection test")
	}
	if !m.connected {
		t.Error("connected should be true after successful test")
	}
	if m.err != nil {
		t.Errorf("err should be nil, got %v", m.err)
	}
}

func TestModel_Update_ConnectionTestedMsg_Error(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	testErr := &testError{msg: "connection failed"}
	updated, _ := model.Update(connectionTestedMsg{err: testErr})
	m := updated.(Model)

	if m.loading {
		t.Error("loading should be false after connection test")
	}
	if m.connected {
		t.Error("connected should be false after failed test")
	}
	if m.err == nil {
		t.Error("err should not be nil after failed test")
	}
}

func TestModel_Update_AccountLoadedMsg(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	account := &api.Account{
		ID:         "123",
		TenantID:   "tenant-1",
		TenantName: "Test Tenant",
	}
	updated, _ := model.Update(common.AccountLoadedMsg{Account: account, Err: nil})
	m := updated.(Model)

	if m.account == nil {
		t.Error("account should not be nil")
	}
	if m.account.TenantName != "Test Tenant" {
		t.Errorf("account.TenantName = %v, want 'Test Tenant'", m.account.TenantName)
	}
}

func TestModel_Update_RefreshAll(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	// Test 'R' refreshes
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if cmd == nil {
		t.Error("'R' should return a batch command for refresh")
	}
}

func TestModel_Update_ComputersLoadedMsg(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	computers := []*protocol.Computer{
		{ID: 1, Name: "Computer1", State: "connected"},
	}
	updated, _ := model.Update(common.ComputersLoadedMsg{Computers: computers})
	m := updated.(Model)

	if m.computers.Count() != 1 {
		t.Errorf("computers count = %d, want 1", m.computers.Count())
	}
}

func TestModel_Update_PrintersLoadedMsg(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	printers := []*protocol.Printer{
		{ID: 1, Name: "Printer1", State: "online"},
	}
	updated, _ := model.Update(common.PrintersLoadedMsg{Printers: printers})
	m := updated.(Model)

	if m.printers.Count() != 1 {
		t.Errorf("printers count = %d, want 1", m.printers.Count())
	}
}

func TestModel_Update_JobsLoadedMsg(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	jobs := []*api.PrintJob{
		{ID: 1, Title: "Job1", State: "done"},
	}
	updated, _ := model.Update(common.JobsLoadedMsg{Jobs: jobs})
	m := updated.(Model)

	if m.jobs.Count() != 1 {
		t.Errorf("jobs count = %d, want 1", m.jobs.Count())
	}
}

func TestModel_View(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)

	// Set window size
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := updated.(Model)

	view := m.View()

	// Should contain header
	if !strings.Contains(view, "PrintRelay Admin") {
		t.Error("view should contain 'PrintRelay Admin'")
	}

	// Should contain server URL
	if !strings.Contains(view, "example.com") {
		t.Error("view should contain server URL")
	}
}

func TestModel_View_Connected(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)
	updated, _ := model.Update(connectionTestedMsg{err: nil})
	updated, _ = updated.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := updated.(Model)

	view := m.View()

	if !strings.Contains(view, "CONNECTED") {
		t.Error("view should show CONNECTED status")
	}
}

func TestModel_View_Disconnected(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)
	updated, _ := model.Update(connectionTestedMsg{err: &testError{msg: "failed"}})
	updated, _ = updated.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := updated.(Model)

	view := m.View()

	if !strings.Contains(view, "DISCONNECTED") {
		t.Error("view should show DISCONNECTED status")
	}
}

func TestModel_View_WithTenantName(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)
	updated, _ := model.Update(common.AccountLoadedMsg{
		Account: &api.Account{TenantName: "My Tenant"},
		Err:     nil,
	})
	updated, _ = updated.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := updated.(Model)

	view := m.View()

	if !strings.Contains(view, "My Tenant") {
		t.Error("view should show tenant name")
	}
}

func TestModel_View_StatusBar(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := updated.(Model)

	view := m.View()

	// Status bar should show keybindings
	if !strings.Contains(view, "j/k") {
		t.Error("status bar should show j/k")
	}
	if !strings.Contains(view, "navigate") {
		t.Error("status bar should show navigate")
	}
	if !strings.Contains(view, "quit") {
		t.Error("status bar should show quit")
	}
}

func TestModel_View_Error(t *testing.T) {
	cfg := Config{
		ServerURL: "https://example.com",
		APIKey:    "test-api-key",
	}
	model, _ := New(cfg)
	updated, _ := model.Update(connectionTestedMsg{err: &testError{msg: "test error message"}})
	updated, _ = updated.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := updated.(Model)

	view := m.View()

	if !strings.Contains(view, "Error") {
		t.Error("view should show error indicator")
	}
}

func TestTabConstants(t *testing.T) {
	if TabComputers != 0 {
		t.Errorf("TabComputers = %d, want 0", TabComputers)
	}
	if TabPrinters != 1 {
		t.Errorf("TabPrinters = %d, want 1", TabPrinters)
	}
	if TabJobs != 2 {
		t.Errorf("TabJobs = %d, want 2", TabJobs)
	}
	if TabTenants != 3 {
		t.Errorf("TabTenants = %d, want 3", TabTenants)
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
