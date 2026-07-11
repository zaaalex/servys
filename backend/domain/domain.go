// Package domain — замороженные доменные типы (контракт §4.B спеки).
// Меняется только по общему согласию команды.
package domain

import "time"

type TenantType string

const (
	TenantB2C TenantType = "b2c"
	TenantB2B TenantType = "b2b"
)

type Tenant struct {
	ID   string
	Type TenantType
}

// User — пользователь по browser-token (X-Client-ID), MVP-идентификация.
type User struct {
	ID         string
	ClientKey  string
	TenantType TenantType
	CreatedAt  time.Time
}

// Vehicle — авто в гараже пользователя. VIN может быть пустым (ручной ввод).
type Vehicle struct {
	ID                   string
	UserID               string
	VIN                  string
	Make                 string
	Model                string
	Year                 int
	EngineCC             int
	PowerHP              int
	Color                string
	BodyType             string
	IdentificationSource string // "drom" | "manual"
	CurrentOdometer      int
	OdometerUpdatedAt    time.Time
}

// Rule — правило регламента (из YAML или LLM). verified=false => демо-данные.
type Rule struct {
	Code           string
	Title          string
	Operation      string // "replace" | "inspect" | ...
	IntervalKm     int
	IntervalMonths int
	LeadKm         int
	Verified       bool
	Source         string
}

type Severity string

const (
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

// Типы событий/статусов alert (ADR-001 §5.6). Type несёт жизненный цикл ТО.
const (
	AlertOdometerUpdateRequired     = "ODOMETER_UPDATE_REQUIRED"
	AlertMaintenanceHistoryRequired = "MAINTENANCE_HISTORY_REQUIRED"
	AlertMaintenanceSoon            = "MAINTENANCE_SOON"
	AlertMaintenanceDue             = "MAINTENANCE_DUE"
	AlertMaintenanceOverdue         = "MAINTENANCE_OVERDUE"
	AlertRegulationNotFound         = "REGULATION_NOT_FOUND"
)

// Alert — рекомендация/статус по авто (то, что отдаёт GET /vehicles/{id}/alerts).
type Alert struct {
	ID          string
	VehicleID   string
	RuleCode    string
	Type        string // одно из Alert* констант выше
	Severity    Severity
	Title       string
	Description string
	DueAtKm     int
}
