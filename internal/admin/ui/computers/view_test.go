package computers

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"printrelay/internal/admin/api"
	"printrelay/internal/admin/ui/common"
	"printrelay/protocol"
)

func TestNew(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	if model.client == nil {
		t.Error("client should not be nil")
	}
	if model.table == nil {
		t.Error("table should not be nil")
	}
	if !model.loading {
		t.Error("model should start in loading state")
	}
	if model.err != nil {
		t.Error("model should not have error initially")
	}
}

func TestModel_Init(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	cmd := model.Init()
	if cmd == nil {
		t.Error("Init() should return a command")
	}
}

func TestModel_Update_KeyNavigation(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)
	model.loading = false

	// Setup some data
	hostname := "test-host"
	computers := []*protocol.Computer{
		{ID: 1, Name: "Computer1", State: "connected", Hostname: &hostname},
		{ID: 2, Name: "Computer2", State: "disconnected"},
		{ID: 3, Name: "Computer3", State: "connected"},
	}
	model, _ = model.Update(common.ComputersLoadedMsg{Computers: computers})

	// Test move down with 'j'
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if model.table.Cursor != 1 {
		t.Errorf("Cursor after 'j' = %d, want 1", model.table.Cursor)
	}

	// Test move up with 'k'
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if model.table.Cursor != 0 {
		t.Errorf("Cursor after 'k' = %d, want 0", model.table.Cursor)
	}

	// Test move down with arrow key
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if model.table.Cursor != 1 {
		t.Errorf("Cursor after down arrow = %d, want 1", model.table.Cursor)
	}

	// Test move up with arrow key
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	if model.table.Cursor != 0 {
		t.Errorf("Cursor after up arrow = %d, want 0", model.table.Cursor)
	}

	// Test move to bottom with 'G'
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if model.table.Cursor != 2 {
		t.Errorf("Cursor after 'G' = %d, want 2", model.table.Cursor)
	}

	// Test move to top with 'g'
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if model.table.Cursor != 0 {
		t.Errorf("Cursor after 'g' = %d, want 0", model.table.Cursor)
	}
}

func TestModel_Update_Refresh(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)
	model.loading = false

	// Test refresh with 'r'
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !model.loading {
		t.Error("model should be loading after refresh")
	}
	if cmd == nil {
		t.Error("refresh should return a command")
	}
}

func TestModel_Update_ComputersLoadedMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	hostname := "test-host"
	version := "1.0.0"
	ip := "192.168.1.1"
	computers := []*protocol.Computer{
		{ID: 1, Name: "Computer1", State: "connected", Hostname: &hostname, Version: &version, Inet: &ip},
		{ID: 2, Name: "Computer2", State: "disconnected"},
	}

	model, _ = model.Update(common.ComputersLoadedMsg{Computers: computers})

	if model.loading {
		t.Error("loading should be false after ComputersLoadedMsg")
	}
	if len(model.computers) != 2 {
		t.Errorf("computers count = %d, want 2", len(model.computers))
	}
	if model.err != nil {
		t.Errorf("unexpected error: %v", model.err)
	}
}

func TestModel_Update_ComputersLoadedMsg_Error(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	testErr := errors.New("test error")
	model, _ = model.Update(common.ComputersLoadedMsg{Err: testErr})

	if model.loading {
		t.Error("loading should be false after error")
	}
	if model.err != testErr {
		t.Errorf("error = %v, want %v", model.err, testErr)
	}
}

func TestModel_Update_RefreshMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)
	model.loading = false

	model, cmd := model.Update(common.RefreshMsg{})

	if !model.loading {
		t.Error("loading should be true after RefreshMsg")
	}
	if cmd == nil {
		t.Error("RefreshMsg should return a command")
	}
}

func TestModel_View_Loading(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	view := model.View()

	if !strings.Contains(view, "Loading computers") {
		t.Error("loading view should contain 'Loading computers'")
	}
}

func TestModel_View_Error(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)
	model.loading = false
	model.err = errors.New("test error")

	view := model.View()

	if !strings.Contains(view, "Error") {
		t.Error("error view should contain 'Error'")
	}
	if !strings.Contains(view, "test error") {
		t.Error("error view should contain error message")
	}
}

func TestModel_View_WithData(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)
	model.loading = false

	hostname := "test-host"
	computers := []*protocol.Computer{
		{ID: 1, Name: "Computer1", State: "connected", Hostname: &hostname},
	}
	model, _ = model.Update(common.ComputersLoadedMsg{Computers: computers})

	view := model.View()

	// Should show table headers
	if !strings.Contains(view, "ID") {
		t.Error("view should contain 'ID' header")
	}
	if !strings.Contains(view, "Name") {
		t.Error("view should contain 'Name' header")
	}
}

func TestModel_Count(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	if model.Count() != 0 {
		t.Errorf("Count() = %d, want 0", model.Count())
	}

	computers := []*protocol.Computer{
		{ID: 1, Name: "Computer1", State: "connected"},
		{ID: 2, Name: "Computer2", State: "disconnected"},
	}
	model, _ = model.Update(common.ComputersLoadedMsg{Computers: computers})

	if model.Count() != 2 {
		t.Errorf("Count() = %d, want 2", model.Count())
	}
}

func TestModel_SetSize(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	model.SetSize(100, 50)

	if model.width != 100 {
		t.Errorf("width = %d, want 100", model.width)
	}
	if model.height != 50 {
		t.Errorf("height = %d, want 50", model.height)
	}
	if model.table.Width != 100 {
		t.Errorf("table.Width = %d, want 100", model.table.Width)
	}
	if model.table.Height != 46 { // 50 - 4
		t.Errorf("table.Height = %d, want 46", model.table.Height)
	}
}

func TestModel_UpdateTable_NilFields(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	// Test with nil optional fields
	computers := []*protocol.Computer{
		{ID: 1, Name: "Computer1", State: "connected", Hostname: nil, Version: nil, Inet: nil},
	}
	model, _ = model.Update(common.ComputersLoadedMsg{Computers: computers})

	// Should not panic
	view := model.View()
	if view == "" {
		t.Error("view should not be empty")
	}
}
