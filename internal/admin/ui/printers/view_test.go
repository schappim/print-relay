package printers

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
	printers := []*protocol.Printer{
		{ID: 1, Name: "Printer1", ComputerID: 1, State: "online", Default: true},
		{ID: 2, Name: "Printer2", ComputerID: 1, State: "offline", Default: false},
		{ID: 3, Name: "Printer3", ComputerID: 2, State: "online", Default: false},
	}
	model, _ = model.Update(common.PrintersLoadedMsg{Printers: printers})

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

	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !model.loading {
		t.Error("model should be loading after refresh")
	}
	if cmd == nil {
		t.Error("refresh should return a command")
	}
}

func TestModel_Update_PrintersLoadedMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	printers := []*protocol.Printer{
		{
			ID:         1,
			Name:       "Printer1",
			ComputerID: 1,
			State:      "online",
			Default:    true,
			Capabilities: protocol.PrinterCapabilities{
				Color:   true,
				Duplex:  true,
				Collate: true,
			},
		},
		{
			ID:         2,
			Name:       "Printer2",
			ComputerID: 1,
			State:      "offline",
			Default:    false,
			Capabilities: protocol.PrinterCapabilities{
				Color:   false,
				Duplex:  false,
				Collate: false,
			},
		},
	}

	model, _ = model.Update(common.PrintersLoadedMsg{Printers: printers})

	if model.loading {
		t.Error("loading should be false after PrintersLoadedMsg")
	}
	if len(model.printers) != 2 {
		t.Errorf("printers count = %d, want 2", len(model.printers))
	}
	if model.err != nil {
		t.Errorf("unexpected error: %v", model.err)
	}
}

func TestModel_Update_PrintersLoadedMsg_Error(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	testErr := errors.New("test error")
	model, _ = model.Update(common.PrintersLoadedMsg{Err: testErr})

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

	if !strings.Contains(view, "Loading printers") {
		t.Error("loading view should contain 'Loading printers'")
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

	printers := []*protocol.Printer{
		{ID: 1, Name: "Printer1", ComputerID: 1, State: "online", Default: true},
	}
	model, _ = model.Update(common.PrintersLoadedMsg{Printers: printers})

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

	printers := []*protocol.Printer{
		{ID: 1, Name: "Printer1", ComputerID: 1, State: "online"},
		{ID: 2, Name: "Printer2", ComputerID: 1, State: "offline"},
	}
	model, _ = model.Update(common.PrintersLoadedMsg{Printers: printers})

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

func TestModel_UpdateTable_Capabilities(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	// Test with various capability combinations
	printers := []*protocol.Printer{
		{
			ID:         1,
			Name:       "FullCapabilities",
			ComputerID: 1,
			State:      "online",
			Capabilities: protocol.PrinterCapabilities{
				Color:   true,
				Duplex:  true,
				Collate: true,
			},
		},
		{
			ID:         2,
			Name:       "NoCapabilities",
			ComputerID: 1,
			State:      "offline",
			Capabilities: protocol.PrinterCapabilities{
				Color:   false,
				Duplex:  false,
				Collate: false,
			},
		},
		{
			ID:         3,
			Name:       "ColorOnly",
			ComputerID: 1,
			State:      "online",
			Capabilities: protocol.PrinterCapabilities{
				Color:   true,
				Duplex:  false,
				Collate: false,
			},
		},
	}

	model, _ = model.Update(common.PrintersLoadedMsg{Printers: printers})
	view := model.View()

	// Should render without error
	if view == "" {
		t.Error("view should not be empty")
	}
}

func TestModel_UpdateTable_EmptyState(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	// Test with empty state
	printers := []*protocol.Printer{
		{ID: 1, Name: "Printer1", ComputerID: 1, State: ""},
	}

	model, _ = model.Update(common.PrintersLoadedMsg{Printers: printers})
	view := model.View()

	// Should show UNKNOWN for empty state
	if !strings.Contains(view, "UNKNOWN") {
		t.Error("empty state should show as UNKNOWN")
	}
}

func TestModel_UpdateTable_DefaultPrinter(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	printers := []*protocol.Printer{
		{ID: 1, Name: "DefaultPrinter", ComputerID: 1, State: "online", Default: true},
		{ID: 2, Name: "RegularPrinter", ComputerID: 1, State: "online", Default: false},
	}

	model, _ = model.Update(common.PrintersLoadedMsg{Printers: printers})
	view := model.View()

	// Should show "Yes" for default printer
	if !strings.Contains(view, "Yes") {
		t.Error("default printer should show 'Yes'")
	}
}
