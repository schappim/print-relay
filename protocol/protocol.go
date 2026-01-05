// Package protocol defines the WebSocket communication protocol between
// the PrintRelay cloud server and clients.
package protocol

import (
	"encoding/json"
	"time"
)

// Message types for WebSocket communication
const (
	// Client -> Server messages
	MsgTypeClientHello       = "client_hello"       // Initial registration
	MsgTypeClientHeartbeat   = "client_heartbeat"   // Keep-alive
	MsgTypePrinterUpdate     = "printer_update"     // Printer list changed
	MsgTypePrintJobStatus    = "printjob_status"    // Job status update
	MsgTypeClientGoodbye     = "client_goodbye"     // Graceful disconnect

	// Server -> Client messages
	MsgTypeServerHello       = "server_hello"       // Registration acknowledged
	MsgTypeServerHeartbeat   = "server_heartbeat"   // Keep-alive response
	MsgTypePrintJobRequest   = "printjob_request"   // New print job to execute
	MsgTypePrintJobCancel    = "printjob_cancel"    // Cancel a print job
	MsgTypeRefreshPrinters   = "refresh_printers"   // Request printer refresh
	MsgTypeServerError       = "server_error"       // Error message
)

// Message is the envelope for all WebSocket messages
type Message struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`        // Message ID for request/response correlation
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// NewMessage creates a new message with the given type and payload
func NewMessage(msgType string, payload any) (*Message, error) {
	var payloadBytes json.RawMessage
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	return &Message{
		Type:      msgType,
		Timestamp: time.Now(),
		Payload:   payloadBytes,
	}, nil
}

// ParsePayload unmarshals the payload into the given struct
func (m *Message) ParsePayload(v any) error {
	if m.Payload == nil {
		return nil
	}
	return json.Unmarshal(m.Payload, v)
}

// ClientHello is sent by the client when connecting
type ClientHello struct {
	ClientID    string     `json:"clientId"`              // Unique client identifier
	ClientKey   string     `json:"clientKey"`             // Client authentication key
	Hostname    string     `json:"hostname"`
	Version     string     `json:"version"`
	OS          string     `json:"os"`
	Arch        string     `json:"arch"`
	Printers    []Printer  `json:"printers"`
}

// ServerHello is sent by the server in response to ClientHello
type ServerHello struct {
	Success     bool   `json:"success"`
	ComputerID  int64  `json:"computerId"`
	Message     string `json:"message,omitempty"`
	ServerTime  time.Time `json:"serverTime"`
}

// Heartbeat is used for keep-alive in both directions
type Heartbeat struct {
	Timestamp time.Time `json:"timestamp"`
}

// Computer represents a computer running the client
type Computer struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Inet      *string    `json:"inet"`
	Inet6     *string    `json:"inet6"`
	Hostname  *string    `json:"hostname"`
	Version   *string    `json:"version"`
	JRE       *string    `json:"jre"`
	CreateTS  time.Time  `json:"createTimestamp"`
	State     string     `json:"state"`
}

// PrinterCapabilities describes what a printer can do
type PrinterCapabilities struct {
	Bins      []string            `json:"bins"`
	Collate   bool                `json:"collate"`
	Color     bool                `json:"color"`
	Copies    int                 `json:"copies"`
	Dpis      []string            `json:"dpis"`
	Duplex    bool                `json:"duplex"`
	Extent    [][]int             `json:"extent,omitempty"`
	Medias    []string            `json:"medias,omitempty"`
	NUp       []int               `json:"nup,omitempty"`
	Papers    map[string][]int    `json:"papers"`
	PrintRate *PrintRate          `json:"printrate,omitempty"`
	Supports  map[string]bool     `json:"supports,omitempty"`
}

// PrintRate describes the print speed
type PrintRate struct {
	Unit  string `json:"unit"`
	Rate  int    `json:"rate"`
}

// Printer represents a printer attached to a computer
type Printer struct {
	ID           int64                `json:"id"`
	ComputerID   int64                `json:"computerId"`
	Name         string               `json:"name"`
	Description  *string              `json:"description"`
	Capabilities PrinterCapabilities  `json:"capabilities"`
	Default      bool                 `json:"default"`
	CreateTS     time.Time            `json:"createTimestamp"`
	State        string               `json:"state"`
}

// PrinterUpdate is sent when printer list changes
type PrinterUpdate struct {
	ComputerID int64     `json:"computerId"`
	Printers   []Printer `json:"printers"`
}

// PrintJobRequest is sent from server to client to execute a print job
type PrintJobRequest struct {
	JobID        int64              `json:"jobId"`
	PrinterID    int64              `json:"printerId"`
	PrinterName  string             `json:"printerName"`
	Title        string             `json:"title"`
	ContentType  string             `json:"contentType"`
	Content      string             `json:"content"`        // Base64 or URI
	Source       string             `json:"source"`
	Options      *PrintJobOptions   `json:"options,omitempty"`
	Qty          int                `json:"qty"`
	Auth         *PrintJobAuth      `json:"authentication,omitempty"`
}

// PrintJobOptions represents print options
type PrintJobOptions struct {
	Bin       string `json:"bin,omitempty"`
	Collate   *bool  `json:"collate,omitempty"`
	Color     *bool  `json:"color,omitempty"`
	Copies    int    `json:"copies,omitempty"`
	Dpi       string `json:"dpi,omitempty"`
	Duplex    string `json:"duplex,omitempty"`
	FitToPage *bool  `json:"fit_to_page,omitempty"`
	Media     string `json:"media,omitempty"`
	NUp       int    `json:"nup,omitempty"`
	Pages     string `json:"pages,omitempty"`
	Paper     string `json:"paper,omitempty"`
	Rotate    int    `json:"rotate,omitempty"`
}

// PrintJobAuth represents authentication for fetching content
type PrintJobAuth struct {
	Type     string `json:"type"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// PrintJobStatus is sent from client to server with job status updates
type PrintJobStatus struct {
	JobID      int64     `json:"jobId"`
	State      string    `json:"state"`
	Message    string    `json:"message"`
	Progress   int       `json:"progress,omitempty"`    // 0-100
	Timestamp  time.Time `json:"timestamp"`
}

// PrintJobCancel is sent from server to client to cancel a job
type PrintJobCancel struct {
	JobID int64 `json:"jobId"`
}

// ServerError represents an error from the server
type ServerError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Print job states
const (
	StateNew        = "new"
	StateSent       = "sent_to_client"
	StateReceived   = "received"
	StateDownloading = "downloading"
	StateQueued     = "queued"
	StatePrinting   = "printing"
	StateDone       = "done"
	StateError      = "error"
	StateDeleted    = "deleted"
	StateExpired    = "expired"
	StateCancelled  = "cancelled"
)

// Content types for print jobs
const (
	ContentTypePDFURI     = "pdf_uri"
	ContentTypePDFBase64  = "pdf_base64"
	ContentTypeRawURI     = "raw_uri"
	ContentTypeRawBase64  = "raw_base64"
)

// Computer states
const (
	ComputerStateConnected    = "connected"
	ComputerStateDisconnected = "disconnected"
)

// Printer states
const (
	PrinterStateOnline  = "online"
	PrinterStateOffline = "offline"
)
