package admin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				ServerURL: "https://example.com",
				APIKey:    "test-api-key",
			},
			wantErr: false,
		},
		{
			name: "missing server URL",
			config: Config{
				APIKey: "test-api-key",
			},
			wantErr: true,
			errMsg:  "server URL is required",
		},
		{
			name: "missing API key",
			config: Config{
				ServerURL: "https://example.com",
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
		{
			name:    "empty config",
			config:  Config{},
			wantErr: true,
			errMsg:  "server URL is required",
		},
		{
			name: "with optional fields",
			config: Config{
				ServerURL:    "https://example.com",
				APIKey:       "test-api-key",
				AdminKey:     "admin-key",
				MonitorToken: "monitor-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temp directory for test files
	tmpDir, err := os.MkdirTemp("", "printrelay-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		content     string
		wantErr     bool
		wantServer  string
		wantAPIKey  string
		wantAdmin   string
		wantMonitor string
	}{
		{
			name: "valid yaml config",
			content: `server: https://example.com
apiKey: my-api-key
adminKey: my-admin-key
monitorToken: my-monitor-token
`,
			wantErr:     false,
			wantServer:  "https://example.com",
			wantAPIKey:  "my-api-key",
			wantAdmin:   "my-admin-key",
			wantMonitor: "my-monitor-token",
		},
		{
			name: "minimal config",
			content: `server: https://example.com
apiKey: my-api-key
`,
			wantErr:    false,
			wantServer: "https://example.com",
			wantAPIKey: "my-api-key",
		},
		{
			name:    "invalid yaml",
			content: `server: https://example.com\n  invalid: [`,
			wantErr: true,
		},
		{
			name:       "empty values",
			content:    `server: ""`,
			wantErr:    false,
			wantServer: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test config file
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			cfg, err := LoadConfig(configPath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadConfig() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("LoadConfig() unexpected error = %v", err)
				return
			}

			if cfg.ServerURL != tt.wantServer {
				t.Errorf("ServerURL = %v, want %v", cfg.ServerURL, tt.wantServer)
			}
			if cfg.APIKey != tt.wantAPIKey {
				t.Errorf("APIKey = %v, want %v", cfg.APIKey, tt.wantAPIKey)
			}
			if cfg.AdminKey != tt.wantAdmin {
				t.Errorf("AdminKey = %v, want %v", cfg.AdminKey, tt.wantAdmin)
			}
			if cfg.MonitorToken != tt.wantMonitor {
				t.Errorf("MonitorToken = %v, want %v", cfg.MonitorToken, tt.wantMonitor)
			}
		})
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("LoadConfig() expected error for non-existent file")
	}
}

func TestFindConfigFile(t *testing.T) {
	// Create a temp directory for test files
	tmpDir, err := os.MkdirTemp("", "printrelay-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Note: FindConfigFile may find existing config files in standard locations
	// (e.g., ~/.config/printrelay/admin.yaml) so we can't test for empty result

	// Create a config file in current directory and test that it's found first
	configPath := filepath.Join(tmpDir, "printrelay-admin.yaml")
	if err := os.WriteFile(configPath, []byte("server: test"), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	result := FindConfigFile()
	// Should find the local config file
	if result != "./printrelay-admin.yaml" {
		t.Errorf("FindConfigFile() = %v, want ./printrelay-admin.yaml (local config should take precedence)", result)
	}
}

func TestFindConfigFile_YmlExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "printrelay-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create a .yml file
	configPath := filepath.Join(tmpDir, "printrelay-admin.yml")
	if err := os.WriteFile(configPath, []byte("server: test"), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	result := FindConfigFile()
	if result != "./printrelay-admin.yml" {
		t.Errorf("FindConfigFile() = %v, want ./printrelay-admin.yml", result)
	}
}
