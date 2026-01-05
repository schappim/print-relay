package cloudserver

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"printrelay/protocol"
)

// ClientConnection represents a connected client
type ClientConnection struct {
	ID         string
	ComputerID int64
	Conn       *websocket.Conn
	Send       chan []byte
	Hub        *Hub
	mu         sync.Mutex
	closed     bool
}

// Hub manages all WebSocket client connections
type Hub struct {
	// Registered clients by client ID
	clients map[string]*ClientConnection

	// Map computer ID to client connection
	computerToClient map[int64]*ClientConnection

	// Inbound messages from clients
	incoming chan *ClientMessage

	// Register requests from clients
	register chan *ClientConnection

	// Unregister requests from clients
	unregister chan *ClientConnection

	// Reference to the store
	store *Store

	mu sync.RWMutex
}

// ClientMessage wraps a message with its source client
type ClientMessage struct {
	Client  *ClientConnection
	Message *protocol.Message
}

// NewHub creates a new Hub
func NewHub(store *Store) *Hub {
	return &Hub{
		clients:          make(map[string]*ClientConnection),
		computerToClient: make(map[int64]*ClientConnection),
		incoming:         make(chan *ClientMessage, 256),
		register:         make(chan *ClientConnection),
		unregister:       make(chan *ClientConnection),
		store:            store,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			if client.ComputerID > 0 {
				h.computerToClient[client.ComputerID] = client
			}
			h.mu.Unlock()
			log.Printf("[Hub] Client registered: %s (computer: %d)", client.ID, client.ComputerID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				if client.ComputerID > 0 {
					delete(h.computerToClient, client.ComputerID)
					// Update computer state to disconnected
					h.store.SetComputerState(client.ComputerID, protocol.ComputerStateDisconnected)
				}
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("[Hub] Client unregistered: %s", client.ID)

		case msg := <-h.incoming:
			h.handleMessage(msg)
		}
	}
}

// handleMessage processes an incoming message from a client
func (h *Hub) handleMessage(cm *ClientMessage) {
	switch cm.Message.Type {
	case protocol.MsgTypeClientHello:
		h.handleClientHello(cm)

	case protocol.MsgTypeClientHeartbeat:
		h.handleHeartbeat(cm)

	case protocol.MsgTypePrinterUpdate:
		h.handlePrinterUpdate(cm)

	case protocol.MsgTypePrintJobStatus:
		h.handlePrintJobStatus(cm)

	case protocol.MsgTypeClientGoodbye:
		h.unregister <- cm.Client

	default:
		log.Printf("[Hub] Unknown message type from %s: %s", cm.Client.ID, cm.Message.Type)
	}
}

// handleClientHello processes client registration
func (h *Hub) handleClientHello(cm *ClientMessage) {
	var hello protocol.ClientHello
	if err := cm.Message.ParsePayload(&hello); err != nil {
		log.Printf("[Hub] Failed to parse ClientHello: %v", err)
		h.sendError(cm.Client, "invalid_payload", "Failed to parse hello message")
		return
	}

	// Verify client key
	if !h.store.ValidateClientKey(hello.ClientKey) {
		log.Printf("[Hub] Invalid client key from %s", cm.Client.ID)
		h.sendError(cm.Client, "unauthorized", "Invalid client key")
		return
	}

	// Register or update computer
	computer := h.store.RegisterComputer(hello)
	cm.Client.ComputerID = computer.ID

	// Update the hub's computer mapping
	h.mu.Lock()
	h.computerToClient[computer.ID] = cm.Client
	h.mu.Unlock()

	// Register printers
	for _, p := range hello.Printers {
		p.ComputerID = computer.ID
		h.store.RegisterPrinter(p)
	}

	// Send response
	response := protocol.ServerHello{
		Success:    true,
		ComputerID: computer.ID,
		Message:    "Welcome to PrintRelay",
		ServerTime: time.Now(),
	}

	h.sendMessage(cm.Client, protocol.MsgTypeServerHello, response)
	log.Printf("[Hub] Computer registered: %s (ID: %d) with %d printers", hello.Hostname, computer.ID, len(hello.Printers))
}

// handleHeartbeat responds to client heartbeats
func (h *Hub) handleHeartbeat(cm *ClientMessage) {
	h.sendMessage(cm.Client, protocol.MsgTypeServerHeartbeat, protocol.Heartbeat{
		Timestamp: time.Now(),
	})
}

// handlePrinterUpdate processes printer list updates
func (h *Hub) handlePrinterUpdate(cm *ClientMessage) {
	var update protocol.PrinterUpdate
	if err := cm.Message.ParsePayload(&update); err != nil {
		log.Printf("[Hub] Failed to parse PrinterUpdate: %v", err)
		return
	}

	// Update printers in store
	h.store.UpdatePrinters(update.ComputerID, update.Printers)
	log.Printf("[Hub] Printer update from computer %d: %d printers", update.ComputerID, len(update.Printers))
}

// handlePrintJobStatus processes job status updates
func (h *Hub) handlePrintJobStatus(cm *ClientMessage) {
	var status protocol.PrintJobStatus
	if err := cm.Message.ParsePayload(&status); err != nil {
		log.Printf("[Hub] Failed to parse PrintJobStatus: %v", err)
		return
	}

	h.store.UpdatePrintJobStatus(status)
	log.Printf("[Hub] Job %d status: %s - %s", status.JobID, status.State, status.Message)
}

// SendPrintJob sends a print job to the appropriate client
func (h *Hub) SendPrintJob(job *protocol.PrintJobRequest, printerID int64) error {
	// Find which computer has this printer
	printer := h.store.GetPrinter(printerID)
	if printer == nil {
		return &Error{Code: "printer_not_found", Message: "Printer not found"}
	}

	// Find connected client for this computer
	h.mu.RLock()
	client, ok := h.computerToClient[printer.ComputerID]
	h.mu.RUnlock()

	if !ok || client == nil {
		return &Error{Code: "computer_offline", Message: "Computer is not connected"}
	}

	// Add printer name to the job
	job.PrinterName = printer.Name
	job.PrinterID = printerID

	return h.sendMessage(client, protocol.MsgTypePrintJobRequest, job)
}

// CancelPrintJob sends a cancel request to the client
func (h *Hub) CancelPrintJob(jobID int64, computerID int64) error {
	h.mu.RLock()
	client, ok := h.computerToClient[computerID]
	h.mu.RUnlock()

	if !ok || client == nil {
		return &Error{Code: "computer_offline", Message: "Computer is not connected"}
	}

	return h.sendMessage(client, protocol.MsgTypePrintJobCancel, protocol.PrintJobCancel{
		JobID: jobID,
	})
}

// sendMessage sends a protocol message to a client
func (h *Hub) sendMessage(client *ClientConnection, msgType string, payload any) error {
	msg, err := protocol.NewMessage(msgType, payload)
	if err != nil {
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	if client.closed {
		return &Error{Code: "client_closed", Message: "Client connection is closed"}
	}

	select {
	case client.Send <- data:
		return nil
	default:
		return &Error{Code: "send_buffer_full", Message: "Client send buffer is full"}
	}
}

// sendError sends an error message to a client
func (h *Hub) sendError(client *ClientConnection, code, message string) {
	h.sendMessage(client, protocol.MsgTypeServerError, protocol.ServerError{
		Code:    code,
		Message: message,
	})
}

// GetConnectedComputers returns IDs of all connected computers
func (h *Hub) GetConnectedComputers() []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ids := make([]int64, 0, len(h.computerToClient))
	for id := range h.computerToClient {
		ids = append(ids, id)
	}
	return ids
}

// IsComputerConnected checks if a computer is currently connected
func (h *Hub) IsComputerConnected(computerID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.computerToClient[computerID]
	return ok
}

// Error represents a hub error
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}
