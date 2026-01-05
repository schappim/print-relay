package cloudserver

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"printrelay/protocol"
)

// Store manages persistent data for the cloud server
type Store struct {
	dataDir   string
	adminKey  string  // Master admin key for tenant management

	// Multi-tenant data
	tenants          map[string]*protocol.Tenant     // Tenant ID -> Tenant
	apiKeyToTenant   map[string]string               // API Key -> Tenant ID
	clientKeyToTenant map[string]string              // Client Key -> Tenant ID
	monitorTokenToTenant map[string]string           // Monitor Token -> Tenant ID

	// Data scoped by tenant
	computers   map[string]map[int64]*protocol.Computer  // Tenant ID -> Computer ID -> Computer
	printers    map[string]map[int64]*protocol.Printer   // Tenant ID -> Printer ID -> Printer
	printJobs   map[string]map[int64]*PrintJob           // Tenant ID -> Job ID -> Job

	nextComputerID int64
	nextPrinterID  int64
	nextJobID      int64

	mu sync.RWMutex
}

// PrintJob extends protocol.PrintJobRequest with server-side fields
type PrintJob struct {
	ID          int64                    `json:"id"`
	TenantID    string                   `json:"tenantId,omitempty"`
	PrinterID   int64                    `json:"printerId"`
	ComputerID  int64                    `json:"computerId"`
	Title       string                   `json:"title"`
	ContentType string                   `json:"contentType"`
	Content     string                   `json:"content"`
	Source      string                   `json:"source"`
	Options     *protocol.PrintJobOptions `json:"options,omitempty"`
	Qty         int                      `json:"qty"`
	State       string                   `json:"state"`
	CreateTS    time.Time                `json:"createTimestamp"`
	States      []PrintJobState          `json:"states,omitempty"`
	ExpireAt    *time.Time               `json:"expireAt,omitempty"`
}

// PrintJobState tracks state changes
type PrintJobState struct {
	State     string    `json:"state"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Account represents the API account
type Account struct {
	ID           int64    `json:"id"`
	FirstName    string   `json:"firstname"`
	LastName     string   `json:"lastname"`
	Email        string   `json:"email"`
	CanCreateSub bool     `json:"canCreateSubAccounts"`
	Credits      int      `json:"credits"`
	NumComputers int      `json:"numComputers"`
	TotalPrints  int      `json:"totalPrints"`
	Versions     []string `json:"versions"`
	Connected    []string `json:"connected"`
	State        string   `json:"state"`
	Permissions  []string `json:"permissions"`
}

// Virtual printer constants
const (
	VirtualComputerID = int64(-1)
	VirtualPrinterID  = int64(-1)
	VirtualPrinterName = "PDF_Virtual_Printer"
)

// NewStore creates a new store
func NewStore(dataDir, adminKey string, defaultAPIKey, defaultClientKey string) *Store {
	s := &Store{
		dataDir:              dataDir,
		adminKey:             adminKey,
		tenants:              make(map[string]*protocol.Tenant),
		apiKeyToTenant:       make(map[string]string),
		clientKeyToTenant:    make(map[string]string),
		monitorTokenToTenant: make(map[string]string),
		computers:            make(map[string]map[int64]*protocol.Computer),
		printers:             make(map[string]map[int64]*protocol.Printer),
		printJobs:            make(map[string]map[int64]*PrintJob),
		nextComputerID:       1,
		nextPrinterID:        1,
		nextJobID:            1,
	}

	// Load persistent data
	s.load()

	// Create default tenant if none exist and keys are provided
	if len(s.tenants) == 0 && defaultAPIKey != "" && defaultClientKey != "" {
		s.createDefaultTenant(defaultAPIKey, defaultClientKey)
	}

	// Register virtual printer for each tenant
	for tenantID := range s.tenants {
		s.registerVirtualPrinter(tenantID)
	}

	return s
}

// createDefaultTenant creates the default tenant with provided keys
func (s *Store) createDefaultTenant(apiKey, clientKey string) {
	tenant := &protocol.Tenant{
		ID:           "default",
		Name:         "Default Tenant",
		APIKeys:      []string{apiKey},
		ClientKey:    clientKey,
		MonitorToken: generateToken(),
		CreatedAt:    time.Now(),
		State:        protocol.TenantStateActive,
	}

	s.tenants[tenant.ID] = tenant
	s.apiKeyToTenant[apiKey] = tenant.ID
	s.clientKeyToTenant[clientKey] = tenant.ID
	s.monitorTokenToTenant[tenant.MonitorToken] = tenant.ID

	// Initialize maps for this tenant
	s.computers[tenant.ID] = make(map[int64]*protocol.Computer)
	s.printers[tenant.ID] = make(map[int64]*protocol.Printer)
	s.printJobs[tenant.ID] = make(map[int64]*PrintJob)

	s.save()
}

// generateToken generates a random token
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// registerVirtualPrinter adds a virtual PDF printer to the server for a tenant
func (s *Store) registerVirtualPrinter(tenantID string) {
	hostname := "printrelay-server"
	version := "1.0.0"

	if s.computers[tenantID] == nil {
		s.computers[tenantID] = make(map[int64]*protocol.Computer)
	}
	if s.printers[tenantID] == nil {
		s.printers[tenantID] = make(map[int64]*protocol.Printer)
	}

	// Virtual computer (the server itself)
	s.computers[tenantID][VirtualComputerID] = &protocol.Computer{
		ID:       VirtualComputerID,
		Name:     "PrintRelay Server (Virtual)",
		Hostname: &hostname,
		Version:  &version,
		CreateTS: time.Now(),
		State:    protocol.ComputerStateConnected,
	}

	// Virtual PDF printer
	desc := "Virtual PDF printer - saves files to server disk"
	s.printers[tenantID][VirtualPrinterID] = &protocol.Printer{
		ID:          VirtualPrinterID,
		ComputerID:  VirtualComputerID,
		Name:        VirtualPrinterName,
		Description: &desc,
		Default:     false,
		CreateTS:    time.Now(),
		State:       protocol.PrinterStateOnline,
		Capabilities: protocol.PrinterCapabilities{
			Bins:    []string{"auto"},
			Collate: true,
			Color:   true,
			Copies:  999,
			Dpis:    []string{"300dpi", "600dpi", "1200dpi"},
			Duplex:  true,
			NUp:     []int{1, 2, 4, 6, 9, 16},
			Papers: map[string][]int{
				"Letter": {612, 792},
				"Legal":  {612, 1008},
				"A4":     {595, 842},
				"A3":     {842, 1191},
				"A5":     {420, 595},
			},
			Medias: []string{"plain", "glossy", "matte"},
		},
	}
}

// IsVirtualPrinter checks if a printer ID is the virtual printer
func (s *Store) IsVirtualPrinter(tenantID string, printerID int64) bool {
	return printerID == VirtualPrinterID
}

// GetPDFOutputDir returns the directory where virtual printer saves PDFs
func (s *Store) GetPDFOutputDir() string {
	return filepath.Join(s.dataDir, "pdf_output")
}

// ValidateAPIKey checks if an API key is valid and returns the tenant
func (s *Store) ValidateAPIKey(key string) (*protocol.Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantID, ok := s.apiKeyToTenant[key]
	if !ok {
		return nil, false
	}

	tenant, ok := s.tenants[tenantID]
	if !ok || tenant.State != protocol.TenantStateActive {
		return nil, false
	}

	return tenant, true
}

// ValidateClientKey checks if a client key is valid and returns the tenant
func (s *Store) ValidateClientKey(key string) (*protocol.Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantID, ok := s.clientKeyToTenant[key]
	if !ok {
		return nil, false
	}

	tenant, ok := s.tenants[tenantID]
	if !ok || tenant.State != protocol.TenantStateActive {
		return nil, false
	}

	return tenant, true
}

// ValidateMonitorToken checks if a monitor token is valid and returns the tenant
func (s *Store) ValidateMonitorToken(token string) (*protocol.Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantID, ok := s.monitorTokenToTenant[token]
	if !ok {
		return nil, false
	}

	tenant, ok := s.tenants[tenantID]
	if !ok || tenant.State != protocol.TenantStateActive {
		return nil, false
	}

	return tenant, true
}

// ValidateAdminKey checks if the admin key is valid
func (s *Store) ValidateAdminKey(key string) bool {
	return key == s.adminKey && s.adminKey != ""
}

// RegisterComputer registers or updates a computer for a tenant
func (s *Store) RegisterComputer(tenantID string, hello protocol.ClientHello) *protocol.Computer {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.computers[tenantID] == nil {
		s.computers[tenantID] = make(map[int64]*protocol.Computer)
	}

	// Check if computer already exists (by hostname)
	for _, c := range s.computers[tenantID] {
		if c.Hostname != nil && *c.Hostname == hello.Hostname {
			c.State = protocol.ComputerStateConnected
			version := hello.Version
			c.Version = &version
			return c
		}
	}

	// Create new computer
	computer := &protocol.Computer{
		ID:       s.nextComputerID,
		Name:     hello.Hostname,
		Hostname: &hello.Hostname,
		Version:  &hello.Version,
		JRE:      &hello.Arch,
		CreateTS: time.Now(),
		State:    protocol.ComputerStateConnected,
	}
	s.nextComputerID++
	s.computers[tenantID][computer.ID] = computer

	s.save()
	return computer
}

// SetComputerState updates a computer's state
func (s *Store) SetComputerState(tenantID string, computerID int64, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.computers[tenantID] == nil {
		return
	}

	if computer, ok := s.computers[tenantID][computerID]; ok {
		computer.State = state
		s.save()
	}
}

// GetComputers returns all computers for a tenant
func (s *Store) GetComputers(tenantID string) []*protocol.Computer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.computers[tenantID] == nil {
		return []*protocol.Computer{}
	}

	result := make([]*protocol.Computer, 0, len(s.computers[tenantID]))
	for _, c := range s.computers[tenantID] {
		result = append(result, c)
	}
	return result
}

// GetComputer returns a specific computer for a tenant
func (s *Store) GetComputer(tenantID string, id int64) *protocol.Computer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.computers[tenantID] == nil {
		return nil
	}
	return s.computers[tenantID][id]
}

// DeleteComputer removes a computer and its printers
func (s *Store) DeleteComputer(tenantID string, id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.computers[tenantID] == nil {
		return false
	}

	if _, ok := s.computers[tenantID][id]; !ok {
		return false
	}

	// Delete associated printers
	if s.printers[tenantID] != nil {
		for pid, p := range s.printers[tenantID] {
			if p.ComputerID == id {
				delete(s.printers[tenantID], pid)
			}
		}
	}

	delete(s.computers[tenantID], id)
	s.save()
	return true
}

// RegisterPrinter registers or updates a printer for a tenant
func (s *Store) RegisterPrinter(tenantID string, p protocol.Printer) *protocol.Printer {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.printers[tenantID] == nil {
		s.printers[tenantID] = make(map[int64]*protocol.Printer)
	}

	// Check if printer already exists
	for _, existing := range s.printers[tenantID] {
		if existing.ComputerID == p.ComputerID && existing.Name == p.Name {
			// Update existing printer
			existing.Description = p.Description
			existing.Capabilities = p.Capabilities
			existing.Default = p.Default
			existing.State = p.State
			return existing
		}
	}

	// Create new printer
	printer := &protocol.Printer{
		ID:           s.nextPrinterID,
		ComputerID:   p.ComputerID,
		Name:         p.Name,
		Description:  p.Description,
		Capabilities: p.Capabilities,
		Default:      p.Default,
		CreateTS:     time.Now(),
		State:        p.State,
	}
	s.nextPrinterID++
	s.printers[tenantID][printer.ID] = printer

	s.save()
	return printer
}

// UpdatePrinters updates the printer list for a computer
func (s *Store) UpdatePrinters(tenantID string, computerID int64, printers []protocol.Printer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.printers[tenantID] == nil {
		s.printers[tenantID] = make(map[int64]*protocol.Printer)
	}

	// Mark all printers for this computer as offline
	for _, p := range s.printers[tenantID] {
		if p.ComputerID == computerID {
			p.State = protocol.PrinterStateOffline
		}
	}

	// Update or add printers
	for _, p := range printers {
		p.ComputerID = computerID
		found := false
		for _, existing := range s.printers[tenantID] {
			if existing.ComputerID == computerID && existing.Name == p.Name {
				existing.Description = p.Description
				existing.Capabilities = p.Capabilities
				existing.Default = p.Default
				existing.State = p.State
				found = true
				break
			}
		}
		if !found {
			printer := &protocol.Printer{
				ID:           s.nextPrinterID,
				ComputerID:   p.ComputerID,
				Name:         p.Name,
				Description:  p.Description,
				Capabilities: p.Capabilities,
				Default:      p.Default,
				CreateTS:     time.Now(),
				State:        p.State,
			}
			s.nextPrinterID++
			s.printers[tenantID][printer.ID] = printer
		}
	}

	s.save()
}

// GetPrinters returns all printers for a tenant
func (s *Store) GetPrinters(tenantID string) []*protocol.Printer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.printers[tenantID] == nil {
		return []*protocol.Printer{}
	}

	result := make([]*protocol.Printer, 0, len(s.printers[tenantID]))
	for _, p := range s.printers[tenantID] {
		result = append(result, p)
	}
	return result
}

// GetPrinter returns a specific printer for a tenant
func (s *Store) GetPrinter(tenantID string, id int64) *protocol.Printer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.printers[tenantID] == nil {
		return nil
	}
	return s.printers[tenantID][id]
}

// GetPrintersForComputer returns printers for a specific computer
func (s *Store) GetPrintersForComputer(tenantID string, computerID int64) []*protocol.Printer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.printers[tenantID] == nil {
		return []*protocol.Printer{}
	}

	result := make([]*protocol.Printer, 0)
	for _, p := range s.printers[tenantID] {
		if p.ComputerID == computerID {
			result = append(result, p)
		}
	}
	return result
}

// CreatePrintJob creates a new print job for a tenant
func (s *Store) CreatePrintJob(tenantID string, printerID int64, title, contentType, content, source string, options *protocol.PrintJobOptions, qty int) (*PrintJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.printers[tenantID] == nil {
		return nil, &Error{Code: "printer_not_found", Message: "Printer not found"}
	}

	printer, ok := s.printers[tenantID][printerID]
	if !ok {
		return nil, &Error{Code: "printer_not_found", Message: "Printer not found"}
	}

	if s.printJobs[tenantID] == nil {
		s.printJobs[tenantID] = make(map[int64]*PrintJob)
	}

	job := &PrintJob{
		ID:          s.nextJobID,
		TenantID:    tenantID,
		PrinterID:   printerID,
		ComputerID:  printer.ComputerID,
		Title:       title,
		ContentType: contentType,
		Content:     content,
		Source:      source,
		Options:     options,
		Qty:         qty,
		State:       protocol.StateNew,
		CreateTS:    time.Now(),
		States: []PrintJobState{
			{State: protocol.StateNew, Message: "Print job created", Timestamp: time.Now()},
		},
	}
	s.nextJobID++
	s.printJobs[tenantID][job.ID] = job

	s.save()
	return job, nil
}

// GetPrintJob returns a specific print job for a tenant
func (s *Store) GetPrintJob(tenantID string, id int64) *PrintJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.printJobs[tenantID] == nil {
		return nil
	}
	return s.printJobs[tenantID][id]
}

// GetPrintJobs returns all print jobs for a tenant
func (s *Store) GetPrintJobs(tenantID string) []*PrintJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.printJobs[tenantID] == nil {
		return []*PrintJob{}
	}

	result := make([]*PrintJob, 0, len(s.printJobs[tenantID]))
	for _, j := range s.printJobs[tenantID] {
		result = append(result, j)
	}
	return result
}

// GetPrintJobsForPrinter returns jobs for a specific printer
func (s *Store) GetPrintJobsForPrinter(tenantID string, printerID int64) []*PrintJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.printJobs[tenantID] == nil {
		return []*PrintJob{}
	}

	result := make([]*PrintJob, 0)
	for _, j := range s.printJobs[tenantID] {
		if j.PrinterID == printerID {
			result = append(result, j)
		}
	}
	return result
}

// UpdatePrintJobStatus updates a print job's status and returns previous state
func (s *Store) UpdatePrintJobStatus(tenantID string, status protocol.PrintJobStatus) (previousState string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.printJobs[tenantID] == nil {
		return ""
	}

	job, ok := s.printJobs[tenantID][status.JobID]
	if !ok {
		return ""
	}

	previousState = job.State
	job.State = status.State
	job.States = append(job.States, PrintJobState{
		State:     status.State,
		Message:   status.Message,
		Timestamp: status.Timestamp,
	})

	s.save()
	return previousState
}

// DeletePrintJob removes a print job
func (s *Store) DeletePrintJob(tenantID string, id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.printJobs[tenantID] == nil {
		return false
	}

	if _, ok := s.printJobs[tenantID][id]; !ok {
		return false
	}

	delete(s.printJobs[tenantID], id)
	s.save()
	return true
}

// GetAccount returns the account info for a tenant (for /whoami endpoint)
func (s *Store) GetAccount(tenantID string, connectedIDs []int64) *Account {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenant := s.tenants[tenantID]
	if tenant == nil {
		return nil
	}

	connected := make([]string, 0)
	if s.computers[tenantID] != nil {
		for _, id := range connectedIDs {
			if c, ok := s.computers[tenantID][id]; ok && c.Hostname != nil {
				connected = append(connected, *c.Hostname)
			}
		}
	}

	numComputers := 0
	if s.computers[tenantID] != nil {
		numComputers = len(s.computers[tenantID])
	}

	numJobs := 0
	if s.printJobs[tenantID] != nil {
		numJobs = len(s.printJobs[tenantID])
	}

	return &Account{
		ID:           1,
		FirstName:    tenant.Name,
		LastName:     "",
		Email:        tenantID + "@printrelay",
		CanCreateSub: false,
		Credits:      999999,
		NumComputers: numComputers,
		TotalPrints:  numJobs,
		Versions:     []string{"1.0.0"},
		Connected:    connected,
		State:        tenant.State,
		Permissions:  []string{"Unrestricted"},
	}
}

// Tenant Management

// CreateTenant creates a new tenant
func (s *Store) CreateTenant(name string) *protocol.Tenant {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant := &protocol.Tenant{
		ID:           generateToken()[:16],
		Name:         name,
		APIKeys:      []string{generateToken()},
		ClientKey:    generateToken(),
		MonitorToken: generateToken(),
		CreatedAt:    time.Now(),
		State:        protocol.TenantStateActive,
	}

	s.tenants[tenant.ID] = tenant
	s.apiKeyToTenant[tenant.APIKeys[0]] = tenant.ID
	s.clientKeyToTenant[tenant.ClientKey] = tenant.ID
	s.monitorTokenToTenant[tenant.MonitorToken] = tenant.ID

	// Initialize maps for this tenant
	s.computers[tenant.ID] = make(map[int64]*protocol.Computer)
	s.printers[tenant.ID] = make(map[int64]*protocol.Printer)
	s.printJobs[tenant.ID] = make(map[int64]*PrintJob)

	// Register virtual printer
	s.registerVirtualPrinter(tenant.ID)

	s.save()
	return tenant
}

// GetTenant returns a tenant by ID
func (s *Store) GetTenant(id string) *protocol.Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tenants[id]
}

// GetTenants returns all tenants
func (s *Store) GetTenants() []*protocol.Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*protocol.Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		result = append(result, t)
	}
	return result
}

// DeleteTenant removes a tenant and all its data
func (s *Store) DeleteTenant(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant, ok := s.tenants[id]
	if !ok {
		return false
	}

	// Remove key mappings
	for _, key := range tenant.APIKeys {
		delete(s.apiKeyToTenant, key)
	}
	delete(s.clientKeyToTenant, tenant.ClientKey)
	delete(s.monitorTokenToTenant, tenant.MonitorToken)

	// Remove tenant data
	delete(s.computers, id)
	delete(s.printers, id)
	delete(s.printJobs, id)
	delete(s.tenants, id)

	s.save()
	return true
}

// AddAPIKey adds a new API key to a tenant
func (s *Store) AddAPIKey(tenantID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant, ok := s.tenants[tenantID]
	if !ok {
		return ""
	}

	newKey := generateToken()
	tenant.APIKeys = append(tenant.APIKeys, newKey)
	s.apiKeyToTenant[newKey] = tenantID

	s.save()
	return newKey
}

// RevokeAPIKey removes an API key from a tenant
func (s *Store) RevokeAPIKey(tenantID, apiKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant, ok := s.tenants[tenantID]
	if !ok {
		return false
	}

	// Don't allow revoking the last key
	if len(tenant.APIKeys) <= 1 {
		return false
	}

	for i, key := range tenant.APIKeys {
		if key == apiKey {
			tenant.APIKeys = append(tenant.APIKeys[:i], tenant.APIKeys[i+1:]...)
			delete(s.apiKeyToTenant, apiKey)
			s.save()
			return true
		}
	}
	return false
}

// RotateMonitorToken generates a new monitor token for a tenant
func (s *Store) RotateMonitorToken(tenantID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant, ok := s.tenants[tenantID]
	if !ok {
		return ""
	}

	// Remove old token mapping
	delete(s.monitorTokenToTenant, tenant.MonitorToken)

	// Generate new token
	tenant.MonitorToken = generateToken()
	s.monitorTokenToTenant[tenant.MonitorToken] = tenantID

	s.save()
	return tenant.MonitorToken
}

// RotateClientKey generates a new client key for a tenant
func (s *Store) RotateClientKey(tenantID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant, ok := s.tenants[tenantID]
	if !ok {
		return ""
	}

	// Remove old key mapping
	delete(s.clientKeyToTenant, tenant.ClientKey)

	// Generate new key
	tenant.ClientKey = generateToken()
	s.clientKeyToTenant[tenant.ClientKey] = tenantID

	s.save()
	return tenant.ClientKey
}

// Persistence

type persistedData struct {
	Tenants        map[string]*protocol.Tenant              `json:"tenants"`
	Computers      map[string]map[int64]*protocol.Computer  `json:"computers"`
	Printers       map[string]map[int64]*protocol.Printer   `json:"printers"`
	PrintJobs      map[string]map[int64]*PrintJob           `json:"printJobs"`
	NextComputerID int64                                    `json:"nextComputerId"`
	NextPrinterID  int64                                    `json:"nextPrinterId"`
	NextJobID      int64                                    `json:"nextJobId"`
}

func (s *Store) load() {
	dataFile := filepath.Join(s.dataDir, "store.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return
	}

	var pd persistedData
	if err := json.Unmarshal(data, &pd); err != nil {
		return
	}

	// Load tenants and rebuild key mappings
	if pd.Tenants != nil {
		s.tenants = pd.Tenants
		for _, tenant := range s.tenants {
			for _, key := range tenant.APIKeys {
				s.apiKeyToTenant[key] = tenant.ID
			}
			s.clientKeyToTenant[tenant.ClientKey] = tenant.ID
			s.monitorTokenToTenant[tenant.MonitorToken] = tenant.ID
		}
	}

	if pd.Computers != nil {
		s.computers = pd.Computers
	}
	if pd.Printers != nil {
		s.printers = pd.Printers
	}
	if pd.PrintJobs != nil {
		s.printJobs = pd.PrintJobs
	}

	s.nextComputerID = pd.NextComputerID
	s.nextPrinterID = pd.NextPrinterID
	s.nextJobID = pd.NextJobID

	// Mark all computers as disconnected on startup
	for _, tenantComputers := range s.computers {
		for _, c := range tenantComputers {
			c.State = protocol.ComputerStateDisconnected
		}
	}
	for _, tenantPrinters := range s.printers {
		for _, p := range tenantPrinters {
			p.State = protocol.PrinterStateOffline
		}
	}
}

func (s *Store) save() {
	pd := persistedData{
		Tenants:        s.tenants,
		Computers:      s.computers,
		Printers:       s.printers,
		PrintJobs:      s.printJobs,
		NextComputerID: s.nextComputerID,
		NextPrinterID:  s.nextPrinterID,
		NextJobID:      s.nextJobID,
	}

	data, err := json.MarshalIndent(pd, "", "  ")
	if err != nil {
		return
	}

	os.MkdirAll(s.dataDir, 0755)
	os.WriteFile(filepath.Join(s.dataDir, "store.json"), data, 0644)
}
