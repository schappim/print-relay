package jobs

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
	jobs := []*api.PrintJob{
		{ID: 1, Title: "Job1", PrinterName: "Printer1", State: "done"},
		{ID: 2, Title: "Job2", PrinterName: "Printer1", State: "printing"},
		{ID: 3, Title: "Job3", PrinterName: "Printer2", State: "queued"},
	}
	model, _ = model.Update(common.JobsLoadedMsg{Jobs: jobs})

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

func TestModel_Update_Delete(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)
	model.loading = false

	jobs := []*api.PrintJob{
		{ID: 1, Title: "Job1", PrinterName: "Printer1", State: "done"},
		{ID: 2, Title: "Job2", PrinterName: "Printer1", State: "printing"},
	}
	model, _ = model.Update(common.JobsLoadedMsg{Jobs: jobs})

	// Test delete with 'd'
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Error("delete should return a command")
	}
}

func TestModel_Update_Delete_EmptyList(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)
	model.loading = false
	model, _ = model.Update(common.JobsLoadedMsg{Jobs: []*api.PrintJob{}})

	// Delete on empty list should not return command
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Error("delete on empty list should not return a command")
	}
}

func TestModel_Update_JobsLoadedMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	now := time.Now()
	jobs := []*api.PrintJob{
		{ID: 1, Title: "Job1", PrinterName: "Printer1", State: "done", CreateTS: now},
		{ID: 2, Title: "Job2", PrinterName: "Printer1", State: "printing", CreateTS: now.Add(-time.Hour)},
	}

	model, _ = model.Update(common.JobsLoadedMsg{Jobs: jobs})

	if model.loading {
		t.Error("loading should be false after JobsLoadedMsg")
	}
	if len(model.jobs) != 2 {
		t.Errorf("jobs count = %d, want 2", len(model.jobs))
	}
	if model.err != nil {
		t.Errorf("unexpected error: %v", model.err)
	}
}

func TestModel_Update_JobsLoadedMsg_Error(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	testErr := errors.New("test error")
	model, _ = model.Update(common.JobsLoadedMsg{Err: testErr})

	if model.loading {
		t.Error("loading should be false after error")
	}
	if model.err != testErr {
		t.Errorf("error = %v, want %v", model.err, testErr)
	}
}

func TestModel_Update_JobDeletedMsg(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)
	model.loading = false

	// Successful delete should trigger refresh
	model, cmd := model.Update(common.JobDeletedMsg{JobID: 1, Err: nil})
	if !model.loading {
		t.Error("loading should be true after successful delete")
	}
	if cmd == nil {
		t.Error("should return refresh command after successful delete")
	}
}

func TestModel_Update_JobDeletedMsg_Error(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)
	model.loading = false

	testErr := errors.New("delete error")
	model, _ = model.Update(common.JobDeletedMsg{JobID: 1, Err: testErr})

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

	if !strings.Contains(view, "Loading print jobs") {
		t.Error("loading view should contain 'Loading print jobs'")
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

	jobs := []*api.PrintJob{
		{ID: 1, Title: "Job1", PrinterName: "Printer1", State: "done"},
	}
	model, _ = model.Update(common.JobsLoadedMsg{Jobs: jobs})

	view := model.View()

	// Should show table headers
	if !strings.Contains(view, "ID") {
		t.Error("view should contain 'ID' header")
	}
	if !strings.Contains(view, "Title") {
		t.Error("view should contain 'Title' header")
	}
}

func TestModel_Count(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	if model.Count() != 0 {
		t.Errorf("Count() = %d, want 0", model.Count())
	}

	jobs := []*api.PrintJob{
		{ID: 1, Title: "Job1", State: "done"},
		{ID: 2, Title: "Job2", State: "printing"},
	}
	model, _ = model.Update(common.JobsLoadedMsg{Jobs: jobs})

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

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{
			name: "zero time",
			time: time.Time{},
			want: "-",
		},
		{
			name: "just now",
			time: now.Add(-30 * time.Second),
			want: "just now",
		},
		{
			name: "1 minute ago",
			time: now.Add(-1 * time.Minute),
			want: "1 min ago",
		},
		{
			name: "5 minutes ago",
			time: now.Add(-5 * time.Minute),
			want: "5 mins ago",
		},
		{
			name: "1 hour ago",
			time: now.Add(-1 * time.Hour),
			want: "1 hour ago",
		},
		{
			name: "3 hours ago",
			time: now.Add(-3 * time.Hour),
			want: "3 hours ago",
		},
		{
			name: "1 day ago",
			time: now.Add(-24 * time.Hour),
			want: "1 day ago",
		},
		{
			name: "5 days ago",
			time: now.Add(-5 * 24 * time.Hour),
			want: "5 days ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeTime(tt.time)
			if got != tt.want {
				t.Errorf("formatRelativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestModel_UpdateTable_AllStates(t *testing.T) {
	client := api.NewClient("https://example.com", "api-key", "")
	model := New(client)

	now := time.Now()
	states := []string{"new", "sent_to_client", "received", "downloading", "queued", "printing", "done", "error", "cancelled"}

	var jobs []*api.PrintJob
	for i, state := range states {
		jobs = append(jobs, &api.PrintJob{
			ID:          int64(i + 1),
			Title:       "Job " + state,
			PrinterName: "Printer1",
			State:       state,
			CreateTS:    now,
		})
	}

	model, _ = model.Update(common.JobsLoadedMsg{Jobs: jobs})
	view := model.View()

	// Should render all states without error
	if view == "" {
		t.Error("view should not be empty")
	}
}
