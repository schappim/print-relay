package tenants

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"printrelay/internal/admin/api"
	"printrelay/internal/admin/ui/common"
)

func TestNew(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
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
	if model.mode != ModeList {
		t.Errorf("mode = %v, want ModeList", model.mode)
	}
}

func TestModel_Init(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)

	cmd := model.Init()
	if cmd == nil {
		t.Error("Init() should return a command")
	}
}

func TestModel_Update_KeyNavigation(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.loading = false

	tenants := []*api.Tenant{
		{ID: "1", Name: "Tenant1", State: "active", APIKeys: []string{"key1"}},
		{ID: "2", Name: "Tenant2", State: "active", APIKeys: []string{"key2"}},
		{ID: "3", Name: "Tenant3", State: "inactive", APIKeys: []string{"key3"}},
	}
	model, _ = model.Update(common.TenantsLoadedMsg{Tenants: tenants})

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

func TestModel_Update_EnterDetailMode(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.loading = false

	tenants := []*api.Tenant{
		{ID: "1", Name: "Tenant1", State: "active", APIKeys: []string{"key1"}},
	}
	model, _ = model.Update(common.TenantsLoadedMsg{Tenants: tenants})

	// Press enter to go to detail mode
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if model.mode != ModeDetail {
		t.Errorf("mode = %v, want ModeDetail", model.mode)
	}
	if model.selected == nil {
		t.Error("selected should not be nil in detail mode")
	}
	if model.selected.ID != "1" {
		t.Errorf("selected.ID = %v, want 1", model.selected.ID)
	}
}

func TestModel_Update_EnterCreateMode(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.loading = false
	model, _ = model.Update(common.TenantsLoadedMsg{Tenants: []*api.Tenant{}})

	// Press 'n' to go to create mode
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if model.mode != ModeCreate {
		t.Errorf("mode = %v, want ModeCreate", model.mode)
	}
	if model.inputName != "" {
		t.Errorf("inputName = %q, want empty", model.inputName)
	}
}

func TestModel_Update_CreateMode_Input(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.loading = false
	model.mode = ModeCreate

	// Type some characters
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})

	if model.inputName != "Test" {
		t.Errorf("inputName = %q, want 'Test'", model.inputName)
	}

	// Test backspace
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if model.inputName != "Tes" {
		t.Errorf("inputName after backspace = %q, want 'Tes'", model.inputName)
	}
}

func TestModel_Update_CreateMode_Escape(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeCreate
	model.inputName = "Test"

	// Press escape to cancel
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if model.mode != ModeList {
		t.Errorf("mode = %v, want ModeList after escape", model.mode)
	}
}

func TestModel_Update_CreateMode_Submit(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeCreate
	model.inputName = "NewTenant"

	// Press enter to submit
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Error("submit should return a command")
	}
}

func TestModel_Update_CreateMode_SubmitEmpty(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeCreate
	model.inputName = ""

	// Press enter with empty name should not submit
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Error("empty submit should not return a command")
	}
}

func TestModel_Update_DetailMode_Escape(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeDetail
	model.selected = &api.Tenant{ID: "1", Name: "Test"}
	model.showNewKey = true

	// Press escape to go back
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if model.mode != ModeList {
		t.Errorf("mode = %v, want ModeList after escape", model.mode)
	}
	if model.showNewKey {
		t.Error("showNewKey should be false after escape")
	}
}

func TestModel_Update_DetailMode_AddAPIKey(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeDetail
	model.selected = &api.Tenant{ID: "1", Name: "Test"}

	// Press 'a' to add API key
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if cmd == nil {
		t.Error("add API key should return a command")
	}
}

func TestModel_Update_DetailMode_RotateClientKey(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeDetail
	model.selected = &api.Tenant{ID: "1", Name: "Test"}

	// Press 'c' to rotate client key
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if cmd == nil {
		t.Error("rotate client key should return a command")
	}
}

func TestModel_Update_Delete(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.loading = false

	tenants := []*api.Tenant{
		{ID: "1", Name: "Tenant1", State: "active"},
	}
	model, _ = model.Update(common.TenantsLoadedMsg{Tenants: tenants})

	// Press 'd' to delete
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if cmd == nil {
		t.Error("delete should return a command")
	}
}

func TestModel_Update_TenantsLoadedMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)

	tenants := []*api.Tenant{
		{ID: "1", Name: "Tenant1", State: "active", APIKeys: []string{"key1"}, CreatedAt: time.Now()},
		{ID: "2", Name: "Tenant2", State: "inactive", APIKeys: []string{"key2", "key3"}, CreatedAt: time.Now()},
	}

	model, _ = model.Update(common.TenantsLoadedMsg{Tenants: tenants})

	if model.loading {
		t.Error("loading should be false after TenantsLoadedMsg")
	}
	if len(model.tenants) != 2 {
		t.Errorf("tenants count = %d, want 2", len(model.tenants))
	}
}

func TestModel_Update_TenantsLoadedMsg_Error(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)

	testErr := errors.New("test error")
	model, _ = model.Update(common.TenantsLoadedMsg{Err: testErr})

	if model.loading {
		t.Error("loading should be false after error")
	}
	if model.err != testErr {
		t.Errorf("error = %v, want %v", model.err, testErr)
	}
}

func TestModel_Update_TenantCreatedMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeCreate

	model, cmd := model.Update(common.TenantCreatedMsg{
		Tenant: &api.Tenant{ID: "new", Name: "New"},
		Err:    nil,
	})

	if model.mode != ModeList {
		t.Errorf("mode = %v, want ModeList after create", model.mode)
	}
	if !model.loading {
		t.Error("loading should be true to refresh list")
	}
	if cmd == nil {
		t.Error("should return refresh command")
	}
}

func TestModel_Update_TenantCreatedMsg_Error(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeCreate

	testErr := errors.New("create error")
	model, _ = model.Update(common.TenantCreatedMsg{Err: testErr})

	if model.mode != ModeList {
		t.Errorf("mode = %v, want ModeList after error", model.mode)
	}
	if model.err != testErr {
		t.Errorf("error = %v, want %v", model.err, testErr)
	}
}

func TestModel_Update_TenantDeletedMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.loading = false

	model, cmd := model.Update(common.TenantDeletedMsg{TenantID: "1", Err: nil})

	if !model.loading {
		t.Error("loading should be true after delete")
	}
	if cmd == nil {
		t.Error("should return refresh command")
	}
}

func TestModel_Update_APIKeyAddedMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeDetail

	model, cmd := model.Update(common.APIKeyAddedMsg{
		TenantID: "1",
		APIKey:   "new-api-key",
		Err:      nil,
	})

	if !model.showNewKey {
		t.Error("showNewKey should be true")
	}
	if model.newKeyType != "API Key" {
		t.Errorf("newKeyType = %v, want 'API Key'", model.newKeyType)
	}
	if model.newKeyValue != "new-api-key" {
		t.Errorf("newKeyValue = %v, want 'new-api-key'", model.newKeyValue)
	}
	if cmd == nil {
		t.Error("should return refresh command")
	}
}

func TestModel_Update_ClientKeyRotatedMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeDetail

	model, cmd := model.Update(common.ClientKeyRotatedMsg{
		TenantID:  "1",
		ClientKey: "new-client-key",
		Err:       nil,
	})

	if !model.showNewKey {
		t.Error("showNewKey should be true")
	}
	if model.newKeyType != "Client Key" {
		t.Errorf("newKeyType = %v, want 'Client Key'", model.newKeyType)
	}
	if model.newKeyValue != "new-client-key" {
		t.Errorf("newKeyValue = %v, want 'new-client-key'", model.newKeyValue)
	}
	if cmd == nil {
		t.Error("should return refresh command")
	}
}

func TestModel_View_Loading(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)

	view := model.View()

	if !strings.Contains(view, "Loading tenants") {
		t.Error("loading view should contain 'Loading tenants'")
	}
}

func TestModel_View_NoAdminAccess(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "") // No admin key
	model := New(client)
	model.loading = false

	view := model.View()

	if !strings.Contains(view, "Admin key required") {
		t.Error("view should indicate admin key is required")
	}
}

func TestModel_View_Error(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.loading = false
	model.err = errors.New("test error")

	view := model.View()

	if !strings.Contains(view, "Error") {
		t.Error("error view should contain 'Error'")
	}
}

func TestModel_View_CreateMode(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeCreate
	model.inputName = "Test"

	view := model.View()

	if !strings.Contains(view, "Create New Tenant") {
		t.Error("create view should contain title")
	}
	if !strings.Contains(view, "Name:") {
		t.Error("create view should contain name field")
	}
	if !strings.Contains(view, "Test") {
		t.Error("create view should contain input value")
	}
}

func TestModel_View_DetailMode(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeDetail
	model.selected = &api.Tenant{
		ID:           "1",
		Name:         "Test Tenant",
		State:        "active",
		ClientKey:    "client-key-123",
		MonitorToken: "monitor-token-456",
		APIKeys:      []string{"api-key-1", "api-key-2"},
		CreatedAt:    time.Now(),
	}

	view := model.View()

	if !strings.Contains(view, "Test Tenant") {
		t.Error("detail view should contain tenant name")
	}
	if !strings.Contains(view, "client-key-123") {
		t.Error("detail view should contain client key")
	}
	if !strings.Contains(view, "api-key-1") {
		t.Error("detail view should contain API keys")
	}
}

func TestModel_View_DetailMode_WithNewKey(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeDetail
	model.selected = &api.Tenant{
		ID:        "1",
		Name:      "Test",
		State:     "active",
		ClientKey: "old-key",
	}
	model.showNewKey = true
	model.newKeyType = "API Key"
	model.newKeyValue = "brand-new-api-key"

	view := model.View()

	if !strings.Contains(view, "New API Key") {
		t.Error("detail view should show new key section")
	}
	if !strings.Contains(view, "brand-new-api-key") {
		t.Error("detail view should show new key value")
	}
}

func TestModel_View_DetailMode_NoSelection(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)
	model.mode = ModeDetail
	model.selected = nil

	view := model.View()

	if !strings.Contains(view, "No tenant selected") {
		t.Error("detail view should show no selection message")
	}
}

func TestModel_Count(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)

	if model.Count() != 0 {
		t.Errorf("Count() = %d, want 0", model.Count())
	}

	tenants := []*api.Tenant{
		{ID: "1", Name: "Tenant1"},
		{ID: "2", Name: "Tenant2"},
	}
	model, _ = model.Update(common.TenantsLoadedMsg{Tenants: tenants})

	if model.Count() != 2 {
		t.Errorf("Count() = %d, want 2", model.Count())
	}
}

func TestModel_SetSize(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "admin-key")
	model := New(client)

	model.SetSize(100, 50)

	if model.width != 100 {
		t.Errorf("width = %d, want 100", model.width)
	}
	if model.height != 50 {
		t.Errorf("height = %d, want 50", model.height)
	}
}

func TestMode_Constants(t *testing.T) {
	if ModeList != 0 {
		t.Errorf("ModeList = %d, want 0", ModeList)
	}
	if ModeDetail != 1 {
		t.Errorf("ModeDetail = %d, want 1", ModeDetail)
	}
	if ModeCreate != 2 {
		t.Errorf("ModeCreate = %d, want 2", ModeCreate)
	}
}
