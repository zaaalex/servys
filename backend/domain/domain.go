// Package domain — типы гаража, регламента и входа.
package domain

import "time"

type TenantType string

const (
	TenantB2C TenantType = "b2c"
)

type Tenant struct {
	ID   string
	Type TenantType
}

const (
	SourceManual = "manual"
	SourceDrom   = "drom"
	SourceNHTSA  = "nhtsa"
)

type Vehicle struct {
	ID                   string
	AccountID            string
	VIN                  string
	Make                 string
	Model                string
	Year                 int
	EngineCC             int
	PowerHP              int
	Color                string
	BodyType             string
	IdentificationSource string
	CurrentOdometer      int
	OdometerUpdatedAt    time.Time
}

type ServiceEvent struct {
	ID          string
	VehicleID   string
	RuleCode    string
	Odometer    int
	PerformedAt time.Time
}

type CommunityNote struct {
	RealIntervalKm int
	Note           string
	Source         string
	Reports        int
}

type Rule struct {
	Code           string
	Title          string
	Operation      string
	IntervalKm     int
	IntervalMonths int
	LeadKm         int
	Verified       bool
	Source         string
	Category       string
	Community      *CommunityNote
}

const (
	CategoryPrimary   = "primary"
	CategorySecondary = "secondary"
)

type Severity string

const (
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

const (
	AlertOdometerUpdateRequired     = "ODOMETER_UPDATE_REQUIRED"
	AlertMaintenanceHistoryRequired = "MAINTENANCE_HISTORY_REQUIRED"
	AlertMaintenanceSoon            = "MAINTENANCE_SOON"
	AlertMaintenanceDue             = "MAINTENANCE_DUE"
	AlertMaintenanceOverdue         = "MAINTENANCE_OVERDUE"
	AlertMaintenanceOK              = "MAINTENANCE_OK"
	AlertRegulationNotFound         = "REGULATION_NOT_FOUND"
)

type Alert struct {
	ID          string
	VehicleID   string
	RuleCode    string
	Type        string
	Severity    Severity
	Title       string
	Description string
	DueAtKm     int
	Category    string
	Community   *CommunityNote
	Basis       string
}
