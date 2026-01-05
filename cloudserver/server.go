package cloudserver

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"printrelay/protocol"
)

// Config holds server configuration
type Config struct {
	APIKeys   []string // API keys for HTTP API authentication
	ClientKey string   // Key for client WebSocket authentication
	DataDir   string
	Verbose   bool
}

// Server represents the PrintRelay cloud server
type Server struct {
	config   Config
	store    *Store
	hub      *Hub
	upgrader websocket.Upgrader
}

// New creates a new PrintRelay cloud server
func New(cfg Config) (*Server, error) {
	store := NewStore(cfg.DataDir, cfg.ClientKey, cfg.APIKeys)
	hub := NewHub(store)

	s := &Server{
		config: cfg,
		store:  store,
		hub:    hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
	}

	// Start the hub
	go hub.Run()

	return s, nil
}

// GenerateKey generates a random key
func GenerateKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Handler returns the HTTP handler for the server
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// WebSocket endpoint for clients (uses client key auth)
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Health check (no auth)
	mux.HandleFunc("/ping", s.handlePing)

	// API endpoints (require API key auth)
	mux.HandleFunc("/", s.withAPIAuth(s.handleRoot))

	return s.withLogging(s.withCORS(mux))
}

// Middleware: logging
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		if s.config.Verbose {
			log.Printf("[HTTP] %s %s %d %v", r.Method, r.URL.Path, lrw.statusCode, time.Since(start))
		}
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker for WebSocket support
func (lrw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := lrw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
}

// Middleware: CORS
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Middleware: API key authentication
func (s *Server) withAPIAuth(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := r.BasicAuth()
		if !ok || !s.store.ValidateAPIKey(user) {
			w.Header().Set("WWW-Authenticate", `Basic realm="PrintRelay API"`)
			s.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid API key")
			return
		}
		handler(w, r)
	}
}

// handleWebSocket handles WebSocket connections from clients
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return
	}

	clientID := GenerateKey()[:16]
	client := &ClientConnection{
		ID:   clientID,
		Conn: conn,
		Send: make(chan []byte, 256),
		Hub:  s.hub,
	}

	s.hub.register <- client

	// Start read and write pumps
	go s.writePump(client)
	go s.readPump(client)
}

// readPump reads messages from the WebSocket connection
func (s *Server) readPump(client *ClientConnection) {
	defer func() {
		s.hub.unregister <- client
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(10 * 1024 * 1024) // 10MB max message size
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WebSocket] Read error from %s: %v", client.ID, err)
			}
			break
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[WebSocket] Invalid message from %s: %v", client.ID, err)
			continue
		}

		s.hub.incoming <- &ClientMessage{
			Client:  client,
			Message: &msg,
		}
	}
}

// writePump writes messages to the WebSocket connection
func (s *Server) writePump(client *ClientConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[WebSocket] Write error to %s: %v", client.ID, err)
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleRoot routes API requests
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/" || path == "":
		s.handleIndex(w, r)
	case path == "/whoami":
		s.handleWhoami(w, r)
	case path == "/noop":
		s.handleNoop(w, r)
	case path == "/computers":
		s.handleComputers(w, r)
	case path == "/printers":
		s.handlePrinters(w, r)
	case path == "/printjobs":
		s.handlePrintJobs(w, r)
	case path == "/printjobs/states":
		s.handlePrintJobStates(w, r)
	case strings.HasPrefix(path, "/computers/"):
		s.handleComputersWithID(w, r)
	case strings.HasPrefix(path, "/printers/"):
		s.handlePrintersWithID(w, r)
	case strings.HasPrefix(path, "/printjobs/"):
		s.handlePrintJobsWithID(w, r)
	default:
		s.writeError(w, http.StatusNotFound, "not_found", "Endpoint not found")
	}
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`"pong"`))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"name":    "PrintRelay Server",
		"version": "1.0.0",
	})
}

func (s *Server) handleNoop(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	account := s.store.GetAccount(s.hub.GetConnectedComputers())
	s.writeJSON(w, http.StatusOK, account)
}

func (s *Server) handleComputers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		computers := s.store.GetComputers()
		// Update connection status
		for _, c := range computers {
			if s.hub.IsComputerConnected(c.ID) {
				c.State = protocol.ComputerStateConnected
			} else {
				c.State = protocol.ComputerStateDisconnected
			}
		}
		s.writeJSON(w, http.StatusOK, computers)

	case http.MethodDelete:
		computers := s.store.GetComputers()
		deleted := make([]int64, 0)
		for _, c := range computers {
			if s.store.DeleteComputer(c.ID) {
				deleted = append(deleted, c.ID)
			}
		}
		s.writeJSON(w, http.StatusOK, deleted)

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

func (s *Server) handleComputersWithID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/computers/")
	parts := strings.Split(path, "/")

	ids, err := parseSetRange(parts[0])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "bad_request", "Invalid computer set")
		return
	}

	// Handle /computers/{id}/printers
	if len(parts) >= 2 && parts[1] == "printers" {
		s.handleComputerPrinters(w, r, ids)
		return
	}

	switch r.Method {
	case http.MethodGet:
		computers := make([]*protocol.Computer, 0)
		for _, id := range ids {
			if c := s.store.GetComputer(id); c != nil {
				if s.hub.IsComputerConnected(c.ID) {
					c.State = protocol.ComputerStateConnected
				}
				computers = append(computers, c)
			}
		}
		s.writeJSON(w, http.StatusOK, computers)

	case http.MethodDelete:
		deleted := make([]int64, 0)
		for _, id := range ids {
			if s.store.DeleteComputer(id) {
				deleted = append(deleted, id)
			}
		}
		s.writeJSON(w, http.StatusOK, deleted)

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

func (s *Server) handleComputerPrinters(w http.ResponseWriter, r *http.Request, computerIDs []int64) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	printers := make([]*protocol.Printer, 0)
	for _, cid := range computerIDs {
		printers = append(printers, s.store.GetPrintersForComputer(cid)...)
	}
	s.writeJSON(w, http.StatusOK, printers)
}

func (s *Server) handlePrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}
	printers := s.store.GetPrinters()
	s.writeJSON(w, http.StatusOK, printers)
}

func (s *Server) handlePrintersWithID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/printers/")
	parts := strings.Split(path, "/")

	ids, err := parseSetRange(parts[0])
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "bad_request", "Invalid printer set")
		return
	}

	// Handle /printers/{id}/printjobs
	if len(parts) >= 2 && parts[1] == "printjobs" {
		s.handlePrinterPrintJobs(w, r, ids)
		return
	}

	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	printers := make([]*protocol.Printer, 0)
	for _, id := range ids {
		if p := s.store.GetPrinter(id); p != nil {
			printers = append(printers, p)
		}
	}
	s.writeJSON(w, http.StatusOK, printers)
}

func (s *Server) handlePrinterPrintJobs(w http.ResponseWriter, r *http.Request, printerIDs []int64) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	jobs := make([]*PrintJob, 0)
	for _, pid := range printerIDs {
		jobs = append(jobs, s.store.GetPrintJobsForPrinter(pid)...)
	}
	s.writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handlePrintJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jobs := s.store.GetPrintJobs()
		s.writeJSON(w, http.StatusOK, jobs)

	case http.MethodPost:
		s.handleCreatePrintJob(w, r)

	case http.MethodDelete:
		jobs := s.store.GetPrintJobs()
		deleted := make([]int64, 0)
		for _, j := range jobs {
			if s.store.DeletePrintJob(j.ID) {
				deleted = append(deleted, j.ID)
			}
		}
		s.writeJSON(w, http.StatusOK, deleted)

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

func (s *Server) handlePrintJobsWithID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/printjobs/")

	if path == "states" {
		s.handlePrintJobStates(w, r)
		return
	}

	ids, err := parseSetRange(path)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "bad_request", "Invalid print job set")
		return
	}

	switch r.Method {
	case http.MethodGet:
		jobs := make([]*PrintJob, 0)
		for _, id := range ids {
			if j := s.store.GetPrintJob(id); j != nil {
				jobs = append(jobs, j)
			}
		}
		s.writeJSON(w, http.StatusOK, jobs)

	case http.MethodDelete:
		deleted := make([]int64, 0)
		for _, id := range ids {
			if s.store.DeletePrintJob(id) {
				deleted = append(deleted, id)
			}
		}
		s.writeJSON(w, http.StatusOK, deleted)

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	}
}

func (s *Server) handlePrintJobStates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return
	}

	states := []map[string]string{
		{"code": protocol.StateNew, "description": "Print job created"},
		{"code": protocol.StateSent, "description": "Sent to client"},
		{"code": protocol.StateReceived, "description": "Received by client"},
		{"code": protocol.StateDownloading, "description": "Downloading content"},
		{"code": protocol.StateQueued, "description": "Queued at printer"},
		{"code": protocol.StatePrinting, "description": "Printing"},
		{"code": protocol.StateDone, "description": "Completed"},
		{"code": protocol.StateError, "description": "Error"},
		{"code": protocol.StateCancelled, "description": "Cancelled"},
	}
	s.writeJSON(w, http.StatusOK, states)
}

// PrintJobCreateRequest is the API request to create a print job
type PrintJobCreateRequest struct {
	PrinterID   int64                    `json:"printerId"`
	Title       string                   `json:"title"`
	ContentType string                   `json:"contentType"`
	Content     string                   `json:"content"`
	Source      string                   `json:"source"`
	Options     *protocol.PrintJobOptions `json:"options,omitempty"`
	Qty         int                      `json:"qty"`
}

func (s *Server) handleCreatePrintJob(w http.ResponseWriter, r *http.Request) {
	var req PrintJobCreateRequest

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			s.writeError(w, http.StatusBadRequest, "bad_request", "Failed to parse form")
			return
		}
		req.PrinterID, _ = strconv.ParseInt(r.FormValue("printerId"), 10, 64)
		req.Title = r.FormValue("title")
		req.ContentType = r.FormValue("contentType")
		req.Content = r.FormValue("content")
		req.Source = r.FormValue("source")
		req.Qty, _ = strconv.Atoi(r.FormValue("qty"))
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("Invalid JSON: %v", err))
			return
		}
	}

	// Validate
	if req.PrinterID == 0 {
		s.writeError(w, http.StatusBadRequest, "bad_request", "printerId is required")
		return
	}
	if req.ContentType == "" {
		s.writeError(w, http.StatusBadRequest, "bad_request", "contentType is required")
		return
	}
	if req.Content == "" {
		s.writeError(w, http.StatusBadRequest, "bad_request", "content is required")
		return
	}

	// Defaults
	if req.Title == "" {
		req.Title = "Untitled"
	}
	if req.Source == "" {
		req.Source = "API"
	}
	if req.Qty < 1 {
		req.Qty = 1
	}

	// Check printer exists
	printer := s.store.GetPrinter(req.PrinterID)
	if printer == nil {
		s.writeError(w, http.StatusBadRequest, "bad_request", "Printer not found")
		return
	}

	// Create job in store
	job, err := s.store.CreatePrintJob(req.PrinterID, req.Title, req.ContentType, req.Content, req.Source, req.Options, req.Qty)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// Handle virtual printer (save to disk)
	if s.store.IsVirtualPrinter(req.PrinterID) {
		go s.processVirtualPrintJob(job, &req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "%d", job.ID)
		return
	}

	// Check computer is connected (only for real printers)
	if !s.hub.IsComputerConnected(printer.ComputerID) {
		s.store.UpdatePrintJobStatus(protocol.PrintJobStatus{
			JobID:     job.ID,
			State:     protocol.StateError,
			Message:   "Computer is not connected",
			Timestamp: time.Now(),
		})
		s.writeError(w, http.StatusServiceUnavailable, "computer_offline", "Computer is not connected")
		return
	}

	// Send to client via WebSocket
	jobReq := &protocol.PrintJobRequest{
		JobID:       job.ID,
		PrinterID:   req.PrinterID,
		PrinterName: printer.Name,
		Title:       req.Title,
		ContentType: req.ContentType,
		Content:     req.Content,
		Source:      req.Source,
		Options:     req.Options,
		Qty:         req.Qty,
	}

	if err := s.hub.SendPrintJob(jobReq, req.PrinterID); err != nil {
		// Update job state to error
		s.store.UpdatePrintJobStatus(protocol.PrintJobStatus{
			JobID:     job.ID,
			State:     protocol.StateError,
			Message:   fmt.Sprintf("Failed to send to client: %v", err),
			Timestamp: time.Now(),
		})
		s.writeError(w, http.StatusServiceUnavailable, "send_failed", err.Error())
		return
	}

	// Update state to sent
	s.store.UpdatePrintJobStatus(protocol.PrintJobStatus{
		JobID:     job.ID,
		State:     protocol.StateSent,
		Message:   "Sent to client",
		Timestamp: time.Now(),
	})

	// Return job ID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "%d", job.ID)
}

// Helper functions

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	s.writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func parseSetRange(s string) ([]int64, error) {
	if s == "" {
		return nil, nil
	}

	ids := make([]int64, 0)
	parts := strings.Split(s, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				continue
			}
			start, err := strconv.ParseInt(rangeParts[0], 10, 64)
			if err != nil {
				continue
			}
			end, err := strconv.ParseInt(rangeParts[1], 10, 64)
			if err != nil {
				continue
			}
			for i := start; i <= end; i++ {
				ids = append(ids, i)
			}
		} else {
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// processVirtualPrintJob handles print jobs for the virtual PDF printer
func (s *Server) processVirtualPrintJob(job *PrintJob, req *PrintJobCreateRequest) {
	// Update status
	s.store.UpdatePrintJobStatus(protocol.PrintJobStatus{
		JobID:     job.ID,
		State:     protocol.StateReceived,
		Message:   "Processing on virtual printer",
		Timestamp: time.Now(),
	})

	// Decode content
	var content []byte
	var err error

	switch req.ContentType {
	case protocol.ContentTypePDFBase64, protocol.ContentTypeRawBase64:
		content, err = base64.StdEncoding.DecodeString(req.Content)
	case protocol.ContentTypePDFURI, protocol.ContentTypeRawURI:
		content, err = s.fetchContent(req.Content)
	default:
		err = fmt.Errorf("unsupported content type: %s", req.ContentType)
	}

	if err != nil {
		log.Printf("[VirtualPrinter] Failed to decode content for job %d: %v", job.ID, err)
		s.store.UpdatePrintJobStatus(protocol.PrintJobStatus{
			JobID:     job.ID,
			State:     protocol.StateError,
			Message:   fmt.Sprintf("Failed to decode content: %v", err),
			Timestamp: time.Now(),
		})
		return
	}

	// Create output directory
	outputDir := s.store.GetPDFOutputDir()
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("[VirtualPrinter] Failed to create output dir: %v", err)
		s.store.UpdatePrintJobStatus(protocol.PrintJobStatus{
			JobID:     job.ID,
			State:     protocol.StateError,
			Message:   fmt.Sprintf("Failed to create output directory: %v", err),
			Timestamp: time.Now(),
		})
		return
	}

	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	safeTitle := sanitizeFilename(req.Title)
	ext := ".pdf"
	if strings.Contains(req.ContentType, "raw") {
		ext = ".raw"
	}
	filename := fmt.Sprintf("%s_%d_%s%s", timestamp, job.ID, safeTitle, ext)
	filepath := filepath.Join(outputDir, filename)

	// Update status
	s.store.UpdatePrintJobStatus(protocol.PrintJobStatus{
		JobID:     job.ID,
		State:     protocol.StatePrinting,
		Message:   "Saving to disk",
		Timestamp: time.Now(),
	})

	// Write file
	if err := os.WriteFile(filepath, content, 0644); err != nil {
		log.Printf("[VirtualPrinter] Failed to write file for job %d: %v", job.ID, err)
		s.store.UpdatePrintJobStatus(protocol.PrintJobStatus{
			JobID:     job.ID,
			State:     protocol.StateError,
			Message:   fmt.Sprintf("Failed to save file: %v", err),
			Timestamp: time.Now(),
		})
		return
	}

	log.Printf("[VirtualPrinter] Job %d saved to: %s (%d bytes)", job.ID, filepath, len(content))

	// Update status to done
	s.store.UpdatePrintJobStatus(protocol.PrintJobStatus{
		JobID:     job.ID,
		State:     protocol.StateDone,
		Message:   fmt.Sprintf("Saved to %s", filepath),
		Timestamp: time.Now(),
	})
}

// fetchContent fetches content from a URL
func (s *Server) fetchContent(url string) ([]byte, error) {
	log.Printf("[VirtualPrinter] Fetching content from URL: %s", url)

	// Create custom transport that completely disables HTTP/2
	// by setting TLSNextProto to empty map and not advertising h2 in ALPN
	transport := &http.Transport{
		ForceAttemptHTTP2: false,
		TLSNextProto:      make(map[string]func(authority string, c *tls.Conn) http.RoundTripper), // Disable HTTP/2
		MaxIdleConns:      10,
		IdleConnTimeout:   30 * time.Second,
		TLSClientConfig: &tls.Config{
			NextProtos: []string{"http/1.1"}, // Only advertise HTTP/1.1
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[VirtualPrinter] Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/pdf,*/*")

	log.Printf("[VirtualPrinter] Sending HTTP/1.1 request...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[VirtualPrinter] HTTP request failed: %v", err)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[VirtualPrinter] Response: HTTP/%d.%d %d, Content-Length: %d", resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, resp.ContentLength)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Printf("[VirtualPrinter] Non-OK response body: %s", string(body))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[VirtualPrinter] Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("[VirtualPrinter] Successfully fetched %d bytes", len(content))
	return content, nil
}

// sanitizeFilename removes unsafe characters from filename
func sanitizeFilename(name string) string {
	// Replace unsafe characters
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, name)

	// Limit length
	if len(safe) > 50 {
		safe = safe[:50]
	}

	if safe == "" {
		safe = "untitled"
	}

	return safe
}
