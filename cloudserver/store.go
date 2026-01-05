package cloudserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"printrelay/protocol"
)

// Store manages persistent data for the cloud server
type Store struct {
	dataDir     string
	clientKey   string
	apiKeys     map[string]bool

	computers   map[int64]*protocol.Computer
	printers    map[int64]*protocol.Printer
	printJobs   map[int64]*PrintJob

	nextComputerID int64
	nextPrinterID  int64
	nextJobID      int64

	mu sync.RWMutex
}

// PrintJob extends protocol.PrintJobRequest with server-side fields
type PrintJob struct {
	ID          int64                    `json:"id"`
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
func NewStore(dataDir, clientKey string, apiKeys []string) *Store {
	s := &Store{
		dataDir:        dataDir,
		clientKey:      clientKey,
		apiKeys:        make(map[string]bool),
		computers:      make(map[int64]*protocol.Computer),
		printers:       make(map[int64]*protocol.Printer),
		printJobs:      make(map[int64]*PrintJob),
		nextComputerID: 1,
		nextPrinterID:  1,
		nextJobID:      1,
	}

	// Register API keys
	for _, key := range apiKeys {
		s.apiKeys[key] = true
	}

	// Load persistent data
	s.load()

	// Register virtual computer and printer
	s.registerVirtualPrinter()

	return s
}

// registerVirtualPrinter adds a virtual PDF printer to the server
func (s *Store) registerVirtualPrinter() {
	hostname := "printrelay-server"
	version := "1.0.0"

	// Virtual computer (the server itself)
	s.computers[VirtualComputerID] = &protocol.Computer{
		ID:       VirtualComputerID,
		Name:     "PrintRelay Server (Virtual)",
		Hostname: &hostname,
		Version:  &version,
		CreateTS: time.Now(),
		State:    protocol.ComputerStateConnected,
	}

	// Virtual PDF printer
	desc := "Virtual PDF printer - saves files to server disk"
	s.printers[VirtualPrinterID] = &protocol.Printer{
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
func (s *Store) IsVirtualPrinter(printerID int64) bool {
	return printerID == VirtualPrinterID
}

// GetPDFOutputDir returns the directory where virtual printer saves PDFs
func (s *Store) GetPDFOutputDir() string {
	return filepath.Join(s.dataDir, "pdf_output")
}

// ValidateAPIKey checks if an API key is valid
func (s *Store) ValidateAPIKey(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.apiKeys[key]
}

// ValidateClientKey checks if a client key is valid
func (s *Store) ValidateClientKey(key string) bool {
	return key == s.clientKey
}

// RegisterComputer registers or updates a computer
func (s *Store) RegisterComputer(hello protocol.ClientHello) *protocol.Computer {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if computer already exists (by client ID)
	for _, c := range s.computers {
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
	s.computers[computer.ID] = computer

	s.save()
	return computer
}

// SetComputerState updates a computer's state
func (s *Store) SetComputerState(computerID int64, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if computer, ok := s.computers[computerID]; ok {
		computer.State = state
		s.save()
	}
}

// GetComputers returns all computers
func (s *Store) GetComputers() []*protocol.Computer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*protocol.Computer, 0, len(s.computers))
	for _, c := range s.computers {
		result = append(result, c)
	}
	return result
}

// GetComputer returns a specific computer
func (s *Store) GetComputer(id int64) *protocol.Computer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.computers[id]
}

// DeleteComputer removes a computer and its printers
func (s *Store) DeleteComputer(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.computers[id]; !ok {
		return false
	}

	// Delete associated printers
	for pid, p := range s.printers {
		if p.ComputerID == id {
			delete(s.printers, pid)
		}
	}

	delete(s.computers, id)
	s.save()
	return true
}

// RegisterPrinter registers or updates a printer
func (s *Store) RegisterPrinter(p protocol.Printer) *protocol.Printer {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if printer already exists
	for _, existing := range s.printers {
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
	s.printers[printer.ID] = printer

	s.save()
	return printer
}

// UpdatePrinters updates the printer list for a computer
func (s *Store) UpdatePrinters(computerID int64, printers []protocol.Printer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Mark all printers for this computer as offline
	for _, p := range s.printers {
		if p.ComputerID == computerID {
			p.State = protocol.PrinterStateOffline
		}
	}

	// Update or add printers
	for _, p := range printers {
		p.ComputerID = computerID
		found := false
		for _, existing := range s.printers {
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
			s.printers[printer.ID] = printer
		}
	}

	s.save()
}

// GetPrinters returns all printers
func (s *Store) GetPrinters() []*protocol.Printer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*protocol.Printer, 0, len(s.printers))
	for _, p := range s.printers {
		result = append(result, p)
	}
	return result
}

// GetPrinter returns a specific printer
func (s *Store) GetPrinter(id int64) *protocol.Printer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.printers[id]
}

// GetPrintersForComputer returns printers for a specific computer
func (s *Store) GetPrintersForComputer(computerID int64) []*protocol.Printer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*protocol.Printer, 0)
	for _, p := range s.printers {
		if p.ComputerID == computerID {
			result = append(result, p)
		}
	}
	return result
}

// CreatePrintJob creates a new print job
func (s *Store) CreatePrintJob(printerID int64, title, contentType, content, source string, options *protocol.PrintJobOptions, qty int) (*PrintJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	printer, ok := s.printers[printerID]
	if !ok {
		return nil, &Error{Code: "printer_not_found", Message: "Printer not found"}
	}

	job := &PrintJob{
		ID:          s.nextJobID,
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
	s.printJobs[job.ID] = job

	s.save()
	return job, nil
}

// GetPrintJob returns a specific print job
func (s *Store) GetPrintJob(id int64) *PrintJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.printJobs[id]
}

// GetPrintJobs returns all print jobs
func (s *Store) GetPrintJobs() []*PrintJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PrintJob, 0, len(s.printJobs))
	for _, j := range s.printJobs {
		result = append(result, j)
	}
	return result
}

// GetPrintJobsForPrinter returns jobs for a specific printer
func (s *Store) GetPrintJobsForPrinter(printerID int64) []*PrintJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PrintJob, 0)
	for _, j := range s.printJobs {
		if j.PrinterID == printerID {
			result = append(result, j)
		}
	}
	return result
}

// UpdatePrintJobStatus updates a print job's status
func (s *Store) UpdatePrintJobStatus(status protocol.PrintJobStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.printJobs[status.JobID]
	if !ok {
		return
	}

	job.State = status.State
	job.States = append(job.States, PrintJobState{
		State:     status.State,
		Message:   status.Message,
		Timestamp: status.Timestamp,
	})

	s.save()
}

// DeletePrintJob removes a print job
func (s *Store) DeletePrintJob(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.printJobs[id]; !ok {
		return false
	}

	delete(s.printJobs, id)
	s.save()
	return true
}

// GetAccount returns the account info
func (s *Store) GetAccount(connectedIDs []int64) *Account {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connected := make([]string, 0)
	for _, id := range connectedIDs {
		if c, ok := s.computers[id]; ok && c.Hostname != nil {
			connected = append(connected, *c.Hostname)
		}
	}

	return &Account{
		ID:           1,
		FirstName:    "PrintRelay",
		LastName:     "Server",
		Email:        "admin@localhost",
		CanCreateSub: false,
		Credits:      999999,
		NumComputers: len(s.computers),
		TotalPrints:  len(s.printJobs),
		Versions:     []string{"1.0.0"},
		Connected:    connected,
		State:        "active",
		Permissions:  []string{"Unrestricted"},
	}
}

// Persistence

type persistedData struct {
	Computers      map[int64]*protocol.Computer `json:"computers"`
	Printers       map[int64]*protocol.Printer  `json:"printers"`
	PrintJobs      map[int64]*PrintJob          `json:"printJobs"`
	NextComputerID int64                        `json:"nextComputerId"`
	NextPrinterID  int64                        `json:"nextPrinterId"`
	NextJobID      int64                        `json:"nextJobId"`
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

	s.computers = pd.Computers
	s.printers = pd.Printers
	s.printJobs = pd.PrintJobs
	s.nextComputerID = pd.NextComputerID
	s.nextPrinterID = pd.NextPrinterID
	s.nextJobID = pd.NextJobID

	// Mark all computers as disconnected on startup
	for _, c := range s.computers {
		c.State = protocol.ComputerStateDisconnected
	}
	for _, p := range s.printers {
		p.State = protocol.PrinterStateOffline
	}
}

func (s *Store) save() {
	pd := persistedData{
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
