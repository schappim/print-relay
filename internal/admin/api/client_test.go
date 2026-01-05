package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		serverURL string
		apiKey    string
		adminKey  string
	}{
		{
			name:      "basic client",
			serverURL: "https://example.com",
			apiKey:    "api-key",
			adminKey:  "",
		},
		{
			name:      "with admin key",
			serverURL: "https://example.com/",
			apiKey:    "api-key",
			adminKey:  "admin-key",
		},
		{
			name:      "trailing slash removed",
			serverURL: "https://example.com/",
			apiKey:    "api-key",
			adminKey:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.serverURL, tt.apiKey, tt.adminKey)
			if client == nil {
				t.Fatal("NewClient() returned nil")
			}
			if client.apiKey != tt.apiKey {
				t.Errorf("apiKey = %v, want %v", client.apiKey, tt.apiKey)
			}
			if client.adminKey != tt.adminKey {
				t.Errorf("adminKey = %v, want %v", client.adminKey, tt.adminKey)
			}
			// Check trailing slash is removed
			if client.baseURL == "https://example.com/" {
				t.Error("baseURL should not have trailing slash")
			}
		})
	}
}

func TestClient_HasAdminAccess(t *testing.T) {
	tests := []struct {
		name     string
		adminKey string
		want     bool
	}{
		{
			name:     "with admin key",
			adminKey: "admin-key",
			want:     true,
		},
		{
			name:     "without admin key",
			adminKey: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient("https://example.com", "api-key", tt.adminKey)
			if got := client.HasAdminAccess(); got != tt.want {
				t.Errorf("HasAdminAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_TestConnection(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful connection",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify auth header
				user, _, ok := r.BasicAuth()
				if !ok || user != "api-key" {
					t.Error("Expected basic auth with api-key")
				}
				if r.URL.Path != "/ping" {
					t.Errorf("Expected /ping, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "api-key", "")
			err := client.TestConnection()
			if tt.wantErr {
				if err == nil {
					t.Error("TestConnection() expected error")
				}
			} else {
				if err != nil {
					t.Errorf("TestConnection() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestClient_GetWhoami(t *testing.T) {
	account := Account{
		ID:           "123",
		FirstName:    "John",
		LastName:     "Doe",
		Email:        "john@example.com",
		NumComputers: 5,
		TenantID:     "tenant-1",
		TenantName:   "Test Tenant",
		State:        "active",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/whoami" {
			t.Errorf("Expected /whoami, got %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(account)
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "")
	result, err := client.GetWhoami()
	if err != nil {
		t.Fatalf("GetWhoami() error: %v", err)
	}

	if result.ID != account.ID {
		t.Errorf("ID = %v, want %v", result.ID, account.ID)
	}
	if result.Email != account.Email {
		t.Errorf("Email = %v, want %v", result.Email, account.Email)
	}
	if result.TenantName != account.TenantName {
		t.Errorf("TenantName = %v, want %v", result.TenantName, account.TenantName)
	}
}

func TestClient_GetWhoami_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "")
	_, err := client.GetWhoami()
	if err == nil {
		t.Error("GetWhoami() expected error for 401 response")
	}
}

func TestClient_GetComputers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/computers" {
			t.Errorf("Expected /computers, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"name":"Computer1","state":"connected"},{"id":2,"name":"Computer2","state":"disconnected"}]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "")
	computers, err := client.GetComputers()
	if err != nil {
		t.Fatalf("GetComputers() error: %v", err)
	}

	if len(computers) != 2 {
		t.Errorf("Expected 2 computers, got %d", len(computers))
	}
	if computers[0].Name != "Computer1" {
		t.Errorf("First computer name = %v, want Computer1", computers[0].Name)
	}
}

func TestClient_GetPrinters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/printers" {
			t.Errorf("Expected /printers, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"name":"Printer1","computer":{"id":1},"default":true}]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "")
	printers, err := client.GetPrinters()
	if err != nil {
		t.Fatalf("GetPrinters() error: %v", err)
	}

	if len(printers) != 1 {
		t.Errorf("Expected 1 printer, got %d", len(printers))
	}
	if printers[0].Name != "Printer1" {
		t.Errorf("Printer name = %v, want Printer1", printers[0].Name)
	}
}

func TestClient_GetPrintJobs(t *testing.T) {
	now := time.Now()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/printjobs" {
			t.Errorf("Expected /printjobs, got %s", r.URL.Path)
		}
		jobs := []PrintJob{
			{
				ID:          1,
				PrinterID:   1,
				PrinterName: "Printer1",
				Title:       "Test Job",
				State:       "done",
				CreateTS:    now,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobs)
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "")
	jobs, err := client.GetPrintJobs()
	if err != nil {
		t.Fatalf("GetPrintJobs() error: %v", err)
	}

	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Title != "Test Job" {
		t.Errorf("Job title = %v, want Test Job", jobs[0].Title)
	}
}

func TestClient_DeletePrintJob(t *testing.T) {
	tests := []struct {
		name       string
		jobID      int64
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful delete",
			jobID:      123,
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "not found",
			jobID:      999,
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" {
					t.Errorf("Expected DELETE, got %s", r.Method)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "api-key", "")
			err := client.DeletePrintJob(tt.jobID)
			if tt.wantErr {
				if err == nil {
					t.Error("DeletePrintJob() expected error")
				}
			} else {
				if err != nil {
					t.Errorf("DeletePrintJob() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestClient_GetTenants(t *testing.T) {
	tenants := []*Tenant{
		{
			ID:        "tenant-1",
			Name:      "Tenant 1",
			APIKeys:   []string{"key1", "key2"},
			ClientKey: "client-key",
			State:     "active",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/tenants" {
			t.Errorf("Expected /admin/tenants, got %s", r.URL.Path)
		}
		// Verify admin auth
		user, _, ok := r.BasicAuth()
		if !ok || user != "admin-key" {
			t.Error("Expected admin key auth")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tenants)
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "admin-key")
	result, err := client.GetTenants()
	if err != nil {
		t.Fatalf("GetTenants() error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 tenant, got %d", len(result))
	}
	if result[0].Name != "Tenant 1" {
		t.Errorf("Tenant name = %v, want Tenant 1", result[0].Name)
	}
}

func TestClient_GetTenants_NoAdminKey(t *testing.T) {
	client := NewClient("https://example.com", "api-key", "")
	_, err := client.GetTenants()
	if err == nil {
		t.Error("GetTenants() expected error without admin key")
	}
}

func TestClient_GetTenant(t *testing.T) {
	tenant := Tenant{
		ID:           "tenant-1",
		Name:         "Tenant 1",
		APIKeys:      []string{"key1"},
		ClientKey:    "client-key",
		MonitorToken: "monitor-token",
		State:        "active",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/tenants/tenant-1" {
			t.Errorf("Expected /admin/tenants/tenant-1, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tenant)
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "admin-key")
	result, err := client.GetTenant("tenant-1")
	if err != nil {
		t.Fatalf("GetTenant() error: %v", err)
	}

	if result.ID != "tenant-1" {
		t.Errorf("Tenant ID = %v, want tenant-1", result.ID)
	}
	if result.MonitorToken != "monitor-token" {
		t.Errorf("MonitorToken = %v, want monitor-token", result.MonitorToken)
	}
}

func TestClient_CreateTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/tenants" {
			t.Errorf("Expected /admin/tenants, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Tenant{
			ID:   "new-tenant",
			Name: "New Tenant",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "admin-key")
	result, err := client.CreateTenant("New Tenant")
	if err != nil {
		t.Fatalf("CreateTenant() error: %v", err)
	}

	if result.Name != "New Tenant" {
		t.Errorf("Tenant name = %v, want New Tenant", result.Name)
	}
}

func TestClient_CreateTenant_NoAdminKey(t *testing.T) {
	client := NewClient("https://example.com", "api-key", "")
	_, err := client.CreateTenant("Test")
	if err == nil {
		t.Error("CreateTenant() expected error without admin key")
	}
}

func TestClient_DeleteTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/admin/tenants/tenant-1" {
			t.Errorf("Expected /admin/tenants/tenant-1, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "admin-key")
	err := client.DeleteTenant("tenant-1")
	if err != nil {
		t.Errorf("DeleteTenant() unexpected error: %v", err)
	}
}

func TestClient_DeleteTenant_NoAdminKey(t *testing.T) {
	client := NewClient("https://example.com", "api-key", "")
	err := client.DeleteTenant("tenant-1")
	if err == nil {
		t.Error("DeleteTenant() expected error without admin key")
	}
}

func TestClient_AddAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/admin/tenants/tenant-1/apikeys" {
			t.Errorf("Expected /admin/tenants/tenant-1/apikeys, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"apiKey":"new-api-key-12345"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "admin-key")
	key, err := client.AddAPIKey("tenant-1")
	if err != nil {
		t.Fatalf("AddAPIKey() error: %v", err)
	}

	if key != "new-api-key-12345" {
		t.Errorf("API key = %v, want new-api-key-12345", key)
	}
}

func TestClient_AddAPIKey_NoAdminKey(t *testing.T) {
	client := NewClient("https://example.com", "api-key", "")
	_, err := client.AddAPIKey("tenant-1")
	if err == nil {
		t.Error("AddAPIKey() expected error without admin key")
	}
}

func TestClient_RotateClientKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/admin/tenants/tenant-1/rotate-client-key" {
			t.Errorf("Expected /admin/tenants/tenant-1/rotate-client-key, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"clientKey":"new-client-key-12345"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "admin-key")
	key, err := client.RotateClientKey("tenant-1")
	if err != nil {
		t.Fatalf("RotateClientKey() error: %v", err)
	}

	if key != "new-client-key-12345" {
		t.Errorf("Client key = %v, want new-client-key-12345", key)
	}
}

func TestClient_RotateClientKey_NoAdminKey(t *testing.T) {
	client := NewClient("https://example.com", "api-key", "")
	_, err := client.RotateClientKey("tenant-1")
	if err == nil {
		t.Error("RotateClientKey() expected error without admin key")
	}
}

func TestClient_RotateMonitorToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/admin/tenants/tenant-1/rotate-monitor-token" {
			t.Errorf("Expected /admin/tenants/tenant-1/rotate-monitor-token, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"monitorToken":"new-monitor-token-12345"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "api-key", "admin-key")
	token, err := client.RotateMonitorToken("tenant-1")
	if err != nil {
		t.Fatalf("RotateMonitorToken() error: %v", err)
	}

	if token != "new-monitor-token-12345" {
		t.Errorf("Monitor token = %v, want new-monitor-token-12345", token)
	}
}

func TestClient_RotateMonitorToken_NoAdminKey(t *testing.T) {
	client := NewClient("https://example.com", "api-key", "")
	_, err := client.RotateMonitorToken("tenant-1")
	if err == nil {
		t.Error("RotateMonitorToken() expected error without admin key")
	}
}

func TestClient_ConnectionError(t *testing.T) {
	client := NewClient("http://localhost:99999", "api-key", "")
	err := client.TestConnection()
	if err == nil {
		t.Error("Expected connection error for invalid port")
	}
}
