package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"printrelay/protocol"
)

// Client is an HTTP client for the PrintRelay API
type Client struct {
	baseURL    string
	apiKey     string
	adminKey   string
	httpClient *http.Client
}

// NewClient creates a new API client
func NewClient(serverURL, apiKey, adminKey string) *Client {
	return &Client{
		baseURL:  strings.TrimSuffix(serverURL, "/"),
		apiKey:   apiKey,
		adminKey: adminKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// HasAdminAccess returns true if admin key is configured
func (c *Client) HasAdminAccess() bool {
	return c.adminKey != ""
}

// Account represents the account info from /whoami
type Account struct {
	ID                 string `json:"id"`
	FirstName          string `json:"firstname"`
	LastName           string `json:"lastname"`
	Email              string `json:"email"`
	CanCreateSubAcct   bool   `json:"canCreateSubAccounts"`
	CreatorEmail       string `json:"creatorEmail"`
	ChildAccounts      []any  `json:"childAccounts"`
	Credits            int    `json:"credits"`
	NumComputers       int    `json:"numComputers"`
	TotalPrints        int    `json:"totalPrints"`
	Versions           []any  `json:"versions"`
	Connected          int    `json:"connected"`
	TenantID           string `json:"tenantId"`
	TenantName         string `json:"tenantName"`
	State              string `json:"state"`
}

// PrintJob represents a print job from the API
type PrintJob struct {
	ID          int64     `json:"id"`
	PrinterID   int64     `json:"printerId"`
	PrinterName string    `json:"printerName"`
	ComputerID  int64     `json:"computerId"`
	Title       string    `json:"title"`
	ContentType string    `json:"contentType"`
	Source      string    `json:"source"`
	State       string    `json:"state"`
	Qty         int       `json:"qty"`
	CreateTS    time.Time `json:"createTimestamp"`
}

// doRequest performs an authenticated HTTP request with API key
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.apiKey, "")
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}

// doAdminRequest performs an authenticated HTTP request with admin key
func (c *Client) doAdminRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.adminKey, "")
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}

// TestConnection tests the API connection
func (c *Client) TestConnection() error {
	resp, err := c.doRequest("GET", "/ping", nil)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	return nil
}

// GetWhoami returns account information
func (c *Client) GetWhoami() (*Account, error) {
	resp, err := c.doRequest("GET", "/whoami", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var account Account
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &account, nil
}

// GetComputers returns all computers
func (c *Client) GetComputers() ([]*protocol.Computer, error) {
	resp, err := c.doRequest("GET", "/computers", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var computers []*protocol.Computer
	if err := json.NewDecoder(resp.Body).Decode(&computers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return computers, nil
}

// GetPrinters returns all printers
func (c *Client) GetPrinters() ([]*protocol.Printer, error) {
	resp, err := c.doRequest("GET", "/printers", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var printers []*protocol.Printer
	if err := json.NewDecoder(resp.Body).Decode(&printers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return printers, nil
}

// GetPrintJobs returns all print jobs
func (c *Client) GetPrintJobs() ([]*PrintJob, error) {
	resp, err := c.doRequest("GET", "/printjobs", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var jobs []*PrintJob
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return jobs, nil
}

// DeletePrintJob deletes a print job by ID
func (c *Client) DeletePrintJob(id int64) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/printjobs/%d", id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Tenant represents a tenant/account
type Tenant struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	APIKeys      []string  `json:"apiKeys"`
	ClientKey    string    `json:"clientKey"`
	MonitorToken string    `json:"monitorToken"`
	CreatedAt    time.Time `json:"createdAt"`
	State        string    `json:"state"`
}

// GetTenants returns all tenants (requires admin key)
func (c *Client) GetTenants() ([]*Tenant, error) {
	if c.adminKey == "" {
		return nil, fmt.Errorf("admin key required")
	}

	resp, err := c.doAdminRequest("GET", "/admin/tenants", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var tenants []*Tenant
	if err := json.NewDecoder(resp.Body).Decode(&tenants); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return tenants, nil
}

// GetTenant returns a specific tenant by ID
func (c *Client) GetTenant(id string) (*Tenant, error) {
	if c.adminKey == "" {
		return nil, fmt.Errorf("admin key required")
	}

	resp, err := c.doAdminRequest("GET", "/admin/tenants/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var tenant Tenant
	if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &tenant, nil
}

// CreateTenant creates a new tenant
func (c *Client) CreateTenant(name string) (*Tenant, error) {
	if c.adminKey == "" {
		return nil, fmt.Errorf("admin key required")
	}

	body := strings.NewReader(fmt.Sprintf(`{"name":"%s"}`, name))
	resp, err := c.doAdminRequest("POST", "/admin/tenants", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var tenant Tenant
	if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &tenant, nil
}

// DeleteTenant deletes a tenant
func (c *Client) DeleteTenant(id string) error {
	if c.adminKey == "" {
		return fmt.Errorf("admin key required")
	}

	resp, err := c.doAdminRequest("DELETE", "/admin/tenants/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// AddAPIKey adds a new API key to a tenant
func (c *Client) AddAPIKey(tenantID string) (string, error) {
	if c.adminKey == "" {
		return "", fmt.Errorf("admin key required")
	}

	resp, err := c.doAdminRequest("POST", "/admin/tenants/"+tenantID+"/apikeys", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		APIKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	return result.APIKey, nil
}

// RotateClientKey rotates the client key for a tenant
func (c *Client) RotateClientKey(tenantID string) (string, error) {
	if c.adminKey == "" {
		return "", fmt.Errorf("admin key required")
	}

	resp, err := c.doAdminRequest("POST", "/admin/tenants/"+tenantID+"/rotate-client-key", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ClientKey string `json:"clientKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	return result.ClientKey, nil
}

// RotateMonitorToken rotates the monitor token for a tenant
func (c *Client) RotateMonitorToken(tenantID string) (string, error) {
	if c.adminKey == "" {
		return "", fmt.Errorf("admin key required")
	}

	resp, err := c.doAdminRequest("POST", "/admin/tenants/"+tenantID+"/rotate-monitor-token", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		MonitorToken string `json:"monitorToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	return result.MonitorToken, nil
}
