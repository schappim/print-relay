package client

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"printrelay/protocol"
)

// Config holds client configuration
type Config struct {
	ServerURL  string // WebSocket server URL (e.g., ws://localhost:8080/ws)
	ClientKey  string // Authentication key
	DataDir    string // Directory for temp files
	Verbose    bool
}

// Client represents a PrintRelay client
type Client struct {
	config     Config
	conn       *websocket.Conn
	computerID int64
	printers   []protocol.Printer
	send       chan []byte
	done       chan struct{}
	mu         sync.RWMutex
	connected  bool
	hostname   string
}

// New creates a new PrintRelay client
func New(cfg Config) (*Client, error) {
	hostname, _ := os.Hostname()

	c := &Client{
		config:   cfg,
		send:     make(chan []byte, 256),
		done:     make(chan struct{}),
		hostname: hostname,
	}

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return c, nil
}

// Run starts the client and maintains connection to the server
func (c *Client) Run() error {
	for {
		if err := c.connect(); err != nil {
			log.Printf("[Client] Connection failed: %v", err)
			log.Printf("[Client] Reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}

		// Run until disconnected
		c.runLoop()

		log.Printf("[Client] Disconnected, reconnecting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

// connect establishes a WebSocket connection to the server
func (c *Client) connect() error {
	// Discover printers first
	printers, err := c.discoverPrinters()
	if err != nil {
		log.Printf("[Client] Warning: Failed to discover printers: %v", err)
	}
	c.printers = printers

	// Connect to server
	log.Printf("[Client] Connecting to %s...", c.config.ServerURL)

	conn, _, err := websocket.DefaultDialer.Dial(c.config.ServerURL, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.conn = conn
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	// Send hello
	hello := protocol.ClientHello{
		ClientID:  c.hostname,
		ClientKey: c.config.ClientKey,
		Hostname:  c.hostname,
		Version:   "1.0.0",
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Printers:  printers,
	}

	if err := c.sendMessage(protocol.MsgTypeClientHello, hello); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send hello: %w", err)
	}

	log.Printf("[Client] Connected, sent %d printers", len(printers))
	return nil
}

// runLoop handles the main message loop
func (c *Client) runLoop() {
	// Start heartbeat ticker
	heartbeatTicker := time.NewTicker(30 * time.Second)
	printerRefreshTicker := time.NewTicker(60 * time.Second)
	defer heartbeatTicker.Stop()
	defer printerRefreshTicker.Stop()

	// Start write pump
	go c.writePump()

	// Create channel for incoming messages
	messageChan := make(chan []byte, 10)
	errorChan := make(chan error, 1)

	// Start read pump in goroutine
	go func() {
		for {
			c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			_, data, err := c.conn.ReadMessage()
			if err != nil {
				errorChan <- err
				return
			}
			messageChan <- data
		}
	}()

	// Main loop
	for {
		select {
		case <-c.done:
			return

		case err := <-errorChan:
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Client] Read error: %v", err)
			}
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			return

		case data := <-messageChan:
			var msg protocol.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Printf("[Client] Invalid message: %v", err)
				continue
			}
			c.handleMessage(&msg)

		case <-heartbeatTicker.C:
			c.sendMessage(protocol.MsgTypeClientHeartbeat, protocol.Heartbeat{
				Timestamp: time.Now(),
			})

		case <-printerRefreshTicker.C:
			c.refreshPrinters()
		}
	}
}

// writePump sends messages to the server
func (c *Client) writePump() {
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[Client] Write error: %v", err)
				return
			}
		}
	}
}

// handleMessage processes incoming messages from the server
func (c *Client) handleMessage(msg *protocol.Message) {
	switch msg.Type {
	case protocol.MsgTypeServerHello:
		var hello protocol.ServerHello
		if err := msg.ParsePayload(&hello); err != nil {
			log.Printf("[Client] Failed to parse ServerHello: %v", err)
			return
		}
		if hello.Success {
			c.computerID = hello.ComputerID
			log.Printf("[Client] Registered as computer %d: %s", hello.ComputerID, hello.Message)
		} else {
			log.Printf("[Client] Registration failed: %s", hello.Message)
		}

	case protocol.MsgTypeServerHeartbeat:
		if c.config.Verbose {
			log.Printf("[Client] Heartbeat received")
		}

	case protocol.MsgTypePrintJobRequest:
		var job protocol.PrintJobRequest
		if err := msg.ParsePayload(&job); err != nil {
			log.Printf("[Client] Failed to parse PrintJobRequest: %v", err)
			return
		}
		go c.executePrintJob(&job)

	case protocol.MsgTypePrintJobCancel:
		var cancel protocol.PrintJobCancel
		if err := msg.ParsePayload(&cancel); err != nil {
			log.Printf("[Client] Failed to parse PrintJobCancel: %v", err)
			return
		}
		log.Printf("[Client] Cancel request for job %d (not implemented)", cancel.JobID)

	case protocol.MsgTypeRefreshPrinters:
		c.refreshPrinters()

	case protocol.MsgTypeServerError:
		var errMsg protocol.ServerError
		if err := msg.ParsePayload(&errMsg); err == nil {
			log.Printf("[Client] Server error: %s - %s", errMsg.Code, errMsg.Message)
		}

	default:
		log.Printf("[Client] Unknown message type: %s", msg.Type)
	}
}

// sendMessage sends a message to the server
func (c *Client) sendMessage(msgType string, payload any) error {
	msg, err := protocol.NewMessage(msgType, payload)
	if err != nil {
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("send buffer full")
	}
}

// refreshPrinters discovers printers and sends update to server
func (c *Client) refreshPrinters() {
	printers, err := c.discoverPrinters()
	if err != nil {
		log.Printf("[Client] Failed to refresh printers: %v", err)
		return
	}

	c.printers = printers

	c.sendMessage(protocol.MsgTypePrinterUpdate, protocol.PrinterUpdate{
		ComputerID: c.computerID,
		Printers:   printers,
	})

	if c.config.Verbose {
		log.Printf("[Client] Sent printer update: %d printers", len(printers))
	}
}

// executePrintJob executes a print job
func (c *Client) executePrintJob(job *protocol.PrintJobRequest) {
	log.Printf("[Client] Executing print job %d: %s on %s", job.JobID, job.Title, job.PrinterName)

	// Report received
	c.reportJobStatus(job.JobID, protocol.StateReceived, "Job received", 0)

	// Get content
	content, err := c.getJobContent(job)
	if err != nil {
		log.Printf("[Client] Failed to get content for job %d: %v", job.JobID, err)
		c.reportJobStatus(job.JobID, protocol.StateError, fmt.Sprintf("Failed to get content: %v", err), 0)
		return
	}

	// Check if this is the virtual printer
	if job.PrinterName == VirtualPrinterName {
		c.executeVirtualPrintJob(job, content)
		return
	}

	// Write to temp file
	tempFile, err := c.writeToTempFile(content, job.ContentType)
	if err != nil {
		log.Printf("[Client] Failed to create temp file for job %d: %v", job.JobID, err)
		c.reportJobStatus(job.JobID, protocol.StateError, fmt.Sprintf("Failed to create temp file: %v", err), 0)
		return
	}
	defer os.Remove(tempFile)

	// Report queued
	c.reportJobStatus(job.JobID, protocol.StateQueued, "Queued for printing", 25)

	// Build and execute print command
	args := c.buildLPArgs(job.PrinterName, tempFile, job)

	c.reportJobStatus(job.JobID, protocol.StatePrinting, "Printing", 50)

	for i := 0; i < job.Qty; i++ {
		cmd := exec.Command("lp", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[Client] Print failed for job %d: %v - %s", job.JobID, err, string(output))
			c.reportJobStatus(job.JobID, protocol.StateError, fmt.Sprintf("Print failed: %v", err), 0)
			return
		}
		if c.config.Verbose {
			log.Printf("[Client] lp output: %s", string(output))
		}
	}

	// Report done
	c.reportJobStatus(job.JobID, protocol.StateDone, "Print completed", 100)
	log.Printf("[Client] Job %d completed successfully", job.JobID)
}

// executeVirtualPrintJob saves the print job to a local PDF file
func (c *Client) executeVirtualPrintJob(job *protocol.PrintJobRequest, content []byte) {
	c.reportJobStatus(job.JobID, protocol.StateQueued, "Queued for virtual printing", 25)

	// Create output directory
	outputDir := filepath.Join(c.config.DataDir, "pdf_output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("[Client] Failed to create output dir: %v", err)
		c.reportJobStatus(job.JobID, protocol.StateError, fmt.Sprintf("Failed to create output directory: %v", err), 0)
		return
	}

	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	safeTitle := sanitizeFilename(job.Title)
	ext := ".pdf"
	if strings.Contains(job.ContentType, "raw") {
		ext = ".raw"
	}
	filename := fmt.Sprintf("%s_%d_%s%s", timestamp, job.JobID, safeTitle, ext)
	outputPath := filepath.Join(outputDir, filename)

	c.reportJobStatus(job.JobID, protocol.StatePrinting, "Saving to disk", 50)

	// Write file
	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		log.Printf("[Client] Failed to write file for job %d: %v", job.JobID, err)
		c.reportJobStatus(job.JobID, protocol.StateError, fmt.Sprintf("Failed to save file: %v", err), 0)
		return
	}

	log.Printf("[Client] Virtual printer saved job %d to: %s (%d bytes)", job.JobID, outputPath, len(content))
	c.reportJobStatus(job.JobID, protocol.StateDone, fmt.Sprintf("Saved to %s", outputPath), 100)
}

// sanitizeFilename removes unsafe characters from filename
func sanitizeFilename(name string) string {
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, name)

	if len(safe) > 50 {
		safe = safe[:50]
	}
	if safe == "" {
		safe = "untitled"
	}
	return safe
}

// reportJobStatus sends a status update to the server
func (c *Client) reportJobStatus(jobID int64, state, message string, progress int) {
	c.sendMessage(protocol.MsgTypePrintJobStatus, protocol.PrintJobStatus{
		JobID:     jobID,
		State:     state,
		Message:   message,
		Progress:  progress,
		Timestamp: time.Now(),
	})
}

// getJobContent retrieves the content to print
func (c *Client) getJobContent(job *protocol.PrintJobRequest) ([]byte, error) {
	switch job.ContentType {
	case protocol.ContentTypePDFBase64, protocol.ContentTypeRawBase64:
		return base64.StdEncoding.DecodeString(job.Content)

	case protocol.ContentTypePDFURI, protocol.ContentTypeRawURI:
		return c.fetchContent(job.Content, job.Auth)

	default:
		return nil, fmt.Errorf("unsupported content type: %s", job.ContentType)
	}
}

// fetchContent fetches content from a URI
func (c *Client) fetchContent(uri string, auth *protocol.PrintJobAuth) ([]byte, error) {
	log.Printf("[Client] Fetching content from: %s", uri)

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	if auth != nil && auth.Type == "BasicAuth" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	// Set headers to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/pdf,*/*")

	// Create transport that forces HTTP/1.1 to avoid HTTP/2 stream errors
	transport := &http.Transport{
		ForceAttemptHTTP2: false,
		TLSNextProto:      make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		MaxIdleConns:      10,
		IdleConnTimeout:   30 * time.Second,
		TLSClientConfig: &tls.Config{
			NextProtos: []string{"http/1.1"},
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
	}

	log.Printf("[Client] Sending HTTP/1.1 request...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Client] HTTP request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("[Client] Response: HTTP/%d.%d %d, Content-Length: %d", resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, resp.ContentLength)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("[Client] Successfully fetched %d bytes", len(content))
	return content, nil
}

// writeToTempFile writes content to a temporary file
func (c *Client) writeToTempFile(content []byte, contentType string) (string, error) {
	ext := ".bin"
	if strings.Contains(contentType, "pdf") {
		ext = ".pdf"
	}

	tempDir := filepath.Join(c.config.DataDir, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp(tempDir, "printjob-*"+ext)
	if err != nil {
		return "", err
	}

	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", err
	}

	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// buildLPArgs builds the arguments for the lp command
func (c *Client) buildLPArgs(printerName, filePath string, job *protocol.PrintJobRequest) []string {
	args := []string{"-d", printerName}

	if job.Title != "" {
		args = append(args, "-t", job.Title)
	}

	if job.Options != nil {
		opts := job.Options

		if opts.Copies > 1 {
			args = append(args, "-n", strconv.Itoa(opts.Copies))
		}

		var oArgs []string

		if opts.Collate != nil && *opts.Collate {
			oArgs = append(oArgs, "collate=true")
		}

		if opts.Color != nil {
			if *opts.Color {
				oArgs = append(oArgs, "ColorModel=Color")
			} else {
				oArgs = append(oArgs, "ColorModel=Gray")
			}
		}

		if opts.Duplex != "" {
			switch strings.ToLower(opts.Duplex) {
			case "long-edge", "two-sided-long-edge":
				oArgs = append(oArgs, "sides=two-sided-long-edge")
			case "short-edge", "two-sided-short-edge":
				oArgs = append(oArgs, "sides=two-sided-short-edge")
			case "none", "one-sided":
				oArgs = append(oArgs, "sides=one-sided")
			}
		}

		if opts.Paper != "" {
			oArgs = append(oArgs, fmt.Sprintf("media=%s", opts.Paper))
		}

		if opts.Bin != "" {
			oArgs = append(oArgs, fmt.Sprintf("InputSlot=%s", opts.Bin))
		}

		if opts.Dpi != "" {
			dpi := strings.TrimSuffix(strings.ToLower(opts.Dpi), "dpi")
			oArgs = append(oArgs, fmt.Sprintf("Resolution=%sdpi", dpi))
		}

		if opts.FitToPage != nil && *opts.FitToPage {
			oArgs = append(oArgs, "fit-to-page")
		}

		if opts.NUp > 1 {
			oArgs = append(oArgs, fmt.Sprintf("number-up=%d", opts.NUp))
		}

		if opts.Pages != "" {
			args = append(args, "-P", opts.Pages)
		}

		for _, opt := range oArgs {
			args = append(args, "-o", opt)
		}
	}

	if strings.Contains(job.ContentType, "raw") {
		args = append(args, "-o", "raw")
	}

	args = append(args, filePath)

	return args
}

// VirtualPrinterName is the name of the client-side virtual PDF printer
const VirtualPrinterName = "PDF_Virtual_Printer_Local"

// discoverPrinters discovers all available printers on macOS via CUPS
func (c *Client) discoverPrinters() ([]protocol.Printer, error) {
	cmd := exec.Command("lpstat", "-p", "-d")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lpstat failed: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	var defaultPrinter string
	printerStates := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "system default destination:") {
			defaultPrinter = strings.TrimSpace(strings.TrimPrefix(line, "system default destination:"))
			continue
		}

		if strings.HasPrefix(line, "printer ") {
			// Extract printer name - it's the word immediately after "printer "
			// Format: "printer NAME is idle|disabled|now printing..."
			rest := strings.TrimPrefix(line, "printer ")
			// Find the end of the printer name (first space after the name)
			fields := strings.Fields(rest)
			if len(fields) >= 1 {
				name := fields[0]
				state := protocol.PrinterStateOnline
				if strings.Contains(line, "disabled") {
					state = protocol.PrinterStateOffline
				}
				printerStates[name] = state
			}
		}
	}

	printers := make([]protocol.Printer, 0, len(printerStates)+1)
	var printerID int64 = 1

	// Add virtual PDF printer first
	virtualDesc := "Virtual PDF printer - saves files locally"
	printers = append(printers, protocol.Printer{
		ID:          printerID,
		Name:        VirtualPrinterName,
		Description: &virtualDesc,
		Default:     false,
		State:       protocol.PrinterStateOnline,
		CreateTS:    time.Now(),
		Capabilities: protocol.PrinterCapabilities{
			Bins:    []string{"auto"},
			Collate: true,
			Color:   true,
			Copies:  999,
			Dpis:    []string{"300dpi", "600dpi", "1200dpi"},
			Duplex:  true,
			Papers: map[string][]int{
				"Letter": {612, 792},
				"Legal":  {612, 1008},
				"A4":     {595, 842},
				"A3":     {842, 1191},
				"A5":     {420, 595},
			},
			Medias: []string{"plain", "glossy", "matte"},
			NUp:    []int{1, 2, 4, 6, 9, 16},
		},
	})
	printerID++

	for name, state := range printerStates {
		printer := protocol.Printer{
			ID:       printerID,
			Name:     name,
			Default:  name == defaultPrinter,
			State:    state,
			CreateTS: time.Now(),
			Capabilities: protocol.PrinterCapabilities{
				Bins:    []string{"auto"},
				Collate: true,
				Color:   true,
				Copies:  999,
				Dpis:    []string{"300dpi", "600dpi"},
				Duplex:  false,
				Papers:  make(map[string][]int),
				NUp:     []int{1, 2, 4, 6, 9, 16},
			},
		}

		// Get detailed capabilities
		c.getPrinterCapabilities(&printer)

		// Get description
		descCmd := exec.Command("lpstat", "-v", name)
		if descOutput, err := descCmd.Output(); err == nil {
			desc := strings.TrimSpace(string(descOutput))
			printer.Description = &desc
		}

		printers = append(printers, printer)
		printerID++
	}

	return printers, nil
}

// getPrinterCapabilities gets detailed printer capabilities
func (c *Client) getPrinterCapabilities(printer *protocol.Printer) {
	cmd := exec.Command("lpoptions", "-p", printer.Name, "-l")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		c.parsePrinterOption(printer, line)
	}
}

// parsePrinterOption parses a printer option line
func (c *Client) parsePrinterOption(printer *protocol.Printer, line string) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return
	}

	optionPart := strings.TrimSpace(parts[0])
	valuesPart := strings.TrimSpace(parts[1])

	optionName := optionPart
	if idx := strings.Index(optionPart, "/"); idx > 0 {
		optionName = optionPart[:idx]
	}

	values := strings.Fields(valuesPart)

	switch strings.ToLower(optionName) {
	case "colormodel":
		for _, v := range values {
			v = strings.TrimPrefix(v, "*")
			if strings.Contains(strings.ToLower(v), "color") || strings.Contains(strings.ToLower(v), "rgb") {
				printer.Capabilities.Color = true
			}
		}

	case "duplex", "sides":
		for _, v := range values {
			v = strings.TrimPrefix(v, "*")
			if v != "None" && v != "one-sided" {
				printer.Capabilities.Duplex = true
				break
			}
		}

	case "resolution":
		dpis := make([]string, 0)
		for _, v := range values {
			dpis = append(dpis, strings.TrimPrefix(v, "*"))
		}
		if len(dpis) > 0 {
			printer.Capabilities.Dpis = dpis
		}

	case "inputslot", "mediasource":
		bins := make([]string, 0)
		for _, v := range values {
			bins = append(bins, strings.TrimPrefix(v, "*"))
		}
		if len(bins) > 0 {
			printer.Capabilities.Bins = bins
		}

	case "pagesize", "media":
		for _, v := range values {
			v = strings.TrimPrefix(v, "*")
			switch strings.ToLower(v) {
			case "letter", "us letter":
				printer.Capabilities.Papers[v] = []int{612, 792}
			case "legal", "us legal":
				printer.Capabilities.Papers[v] = []int{612, 1008}
			case "a4":
				printer.Capabilities.Papers[v] = []int{595, 842}
			case "a3":
				printer.Capabilities.Papers[v] = []int{842, 1191}
			case "a5":
				printer.Capabilities.Papers[v] = []int{420, 595}
			default:
				printer.Capabilities.Papers[v] = []int{612, 792}
			}
		}

	case "mediatype":
		medias := make([]string, 0)
		for _, v := range values {
			medias = append(medias, strings.TrimPrefix(v, "*"))
		}
		printer.Capabilities.Medias = medias
	}
}

// ParseServerURL parses and validates the server URL
func ParseServerURL(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}

	// Convert http to ws
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// Already correct
	default:
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	// Ensure /ws path
	if u.Path == "" || u.Path == "/" {
		u.Path = "/ws"
	}

	return u.String(), nil
}
