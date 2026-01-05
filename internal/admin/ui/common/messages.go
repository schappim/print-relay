package common

import (
	"printrelay/internal/admin/api"
	"printrelay/protocol"
)

// Data loading messages
type ComputersLoadedMsg struct {
	Computers []*protocol.Computer
	Err       error
}

type PrintersLoadedMsg struct {
	Printers []*protocol.Printer
	Err      error
}

type JobsLoadedMsg struct {
	Jobs []*api.PrintJob
	Err  error
}

type AccountLoadedMsg struct {
	Account *api.Account
	Err     error
}

type TenantsLoadedMsg struct {
	Tenants []*api.Tenant
	Err     error
}

// Action messages
type RefreshMsg struct{}

type ErrorMsg struct {
	Err error
}

type JobDeletedMsg struct {
	JobID int64
	Err   error
}

type TenantCreatedMsg struct {
	Tenant *api.Tenant
	Err    error
}

type TenantDeletedMsg struct {
	TenantID string
	Err      error
}

type APIKeyAddedMsg struct {
	TenantID string
	APIKey   string
	Err      error
}

type ClientKeyRotatedMsg struct {
	TenantID  string
	ClientKey string
	Err       error
}
