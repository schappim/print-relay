package cloudserver

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"printrelay/protocol"
)

// MonitorConnection represents a connected monitor client
type MonitorConnection struct {
	ID       string
	TenantID string
	Conn     *websocket.Conn
	Send     chan []byte
	Hub      *MonitorHub
	mu       sync.Mutex
	closed   bool
}

// MonitorHub manages all monitor WebSocket connections
type MonitorHub struct {
	// Monitors by tenant ID -> connection ID -> connection
	monitors map[string]map[string]*MonitorConnection

	// Register requests
	register chan *MonitorConnection

	// Unregister requests
	unregister chan *MonitorConnection

	// Broadcast events by tenant
	broadcast chan *TenantEvent

	mu sync.RWMutex
}

// TenantEvent is an event to broadcast to a tenant's monitors
type TenantEvent struct {
	TenantID string
	Event    *protocol.MonitorEvent
}

// NewMonitorHub creates a new MonitorHub
func NewMonitorHub() *MonitorHub {
	return &MonitorHub{
		monitors:   make(map[string]map[string]*MonitorConnection),
		register:   make(chan *MonitorConnection),
		unregister: make(chan *MonitorConnection),
		broadcast:  make(chan *TenantEvent, 256),
	}
}

// Run starts the monitor hub's main loop
func (h *MonitorHub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			if h.monitors[conn.TenantID] == nil {
				h.monitors[conn.TenantID] = make(map[string]*MonitorConnection)
			}
			h.monitors[conn.TenantID][conn.ID] = conn
			h.mu.Unlock()
			log.Printf("[MonitorHub] Monitor registered: %s for tenant %s", conn.ID, conn.TenantID)

		case conn := <-h.unregister:
			h.mu.Lock()
			if tenantMonitors, ok := h.monitors[conn.TenantID]; ok {
				if _, ok := tenantMonitors[conn.ID]; ok {
					delete(tenantMonitors, conn.ID)
					close(conn.Send)
					if len(tenantMonitors) == 0 {
						delete(h.monitors, conn.TenantID)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("[MonitorHub] Monitor unregistered: %s", conn.ID)

		case event := <-h.broadcast:
			h.broadcastToTenant(event)
		}
	}
}

// broadcastToTenant sends an event to all monitors for a tenant
func (h *MonitorHub) broadcastToTenant(event *TenantEvent) {
	h.mu.RLock()
	tenantMonitors, ok := h.monitors[event.TenantID]
	if !ok {
		h.mu.RUnlock()
		return
	}

	// Copy the slice to avoid holding the lock during send
	monitors := make([]*MonitorConnection, 0, len(tenantMonitors))
	for _, m := range tenantMonitors {
		monitors = append(monitors, m)
	}
	h.mu.RUnlock()

	data, err := json.Marshal(event.Event)
	if err != nil {
		log.Printf("[MonitorHub] Failed to marshal event: %v", err)
		return
	}

	for _, m := range monitors {
		m.mu.Lock()
		if !m.closed {
			select {
			case m.Send <- data:
			default:
				// Buffer full, skip this message
				log.Printf("[MonitorHub] Buffer full for monitor %s, dropping message", m.ID)
			}
		}
		m.mu.Unlock()
	}
}

// EmitJobCreated broadcasts a job created event
func (h *MonitorHub) EmitJobCreated(tenantID string, job *PrintJob, printerName string) {
	event := &protocol.MonitorEvent{
		Type:      protocol.MsgTypeJobCreated,
		Timestamp: time.Now(),
		Data: protocol.JobCreatedEvent{
			ID:          job.ID,
			PrinterID:   job.PrinterID,
			PrinterName: printerName,
			Title:       job.Title,
			State:       job.State,
			Source:      job.Source,
		},
	}

	h.broadcast <- &TenantEvent{
		TenantID: tenantID,
		Event:    event,
	}
}

// EmitJobStateChanged broadcasts a job state change event
func (h *MonitorHub) EmitJobStateChanged(tenantID string, jobID, printerID int64, previousState, newState, message string) {
	event := &protocol.MonitorEvent{
		Type:      protocol.MsgTypeJobStateChanged,
		Timestamp: time.Now(),
		Data: protocol.JobStateChangedEvent{
			ID:            jobID,
			PrinterID:     printerID,
			PreviousState: previousState,
			State:         newState,
			Message:       message,
		},
	}

	h.broadcast <- &TenantEvent{
		TenantID: tenantID,
		Event:    event,
	}
}

// EmitPrinterConnected broadcasts a printer connected event
func (h *MonitorHub) EmitPrinterConnected(tenantID string, computer *protocol.Computer, printers []protocol.Printer) {
	computerName := ""
	if computer.Hostname != nil {
		computerName = *computer.Hostname
	}

	event := &protocol.MonitorEvent{
		Type:      protocol.MsgTypePrinterConnected,
		Timestamp: time.Now(),
		Data: protocol.PrinterConnectedEvent{
			ComputerID:   computer.ID,
			ComputerName: computerName,
			Printers:     printers,
		},
	}

	h.broadcast <- &TenantEvent{
		TenantID: tenantID,
		Event:    event,
	}
}

// EmitPrinterDisconnected broadcasts a printer disconnected event
func (h *MonitorHub) EmitPrinterDisconnected(tenantID string, computer *protocol.Computer) {
	computerName := ""
	if computer.Hostname != nil {
		computerName = *computer.Hostname
	}

	event := &protocol.MonitorEvent{
		Type:      protocol.MsgTypePrinterDisconnected,
		Timestamp: time.Now(),
		Data: protocol.PrinterDisconnectedEvent{
			ComputerID:   computer.ID,
			ComputerName: computerName,
		},
	}

	h.broadcast <- &TenantEvent{
		TenantID: tenantID,
		Event:    event,
	}
}

// GetMonitorCount returns the number of monitors for a tenant
func (h *MonitorHub) GetMonitorCount(tenantID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if monitors, ok := h.monitors[tenantID]; ok {
		return len(monitors)
	}
	return 0
}

// SendWelcome sends a welcome message to a newly connected monitor
func (h *MonitorHub) SendWelcome(conn *MonitorConnection, tenant *protocol.Tenant) error {
	welcome := &protocol.MonitorEvent{
		Type:      protocol.MsgTypeMonitorWelcome,
		Timestamp: time.Now(),
		Data: protocol.MonitorWelcome{
			TenantID:   tenant.ID,
			TenantName: tenant.Name,
			ServerTime: time.Now(),
		},
	}

	data, err := json.Marshal(welcome)
	if err != nil {
		return err
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.closed {
		return &Error{Code: "connection_closed", Message: "Connection is closed"}
	}

	select {
	case conn.Send <- data:
		return nil
	default:
		return &Error{Code: "send_buffer_full", Message: "Send buffer is full"}
	}
}
