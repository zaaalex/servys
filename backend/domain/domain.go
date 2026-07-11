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

// ServiceEvent — подтверждённое выполнение работы: baseline для расчёта следующего срока (ADR §8).
type ServiceEvent struct {
	ID          string
	VehicleID   string
	RuleCode    string
	Odometer    int
	PerformedAt time.Time
}

// CommunityNote — данные из отзывов/форумов владельцев (НЕ официальный регламент).
// Показывается рядом с регламентом как «народное» основание. Заполняется demo-данными
// (боевой парсинг отзывов из LLM-пайплайна — следующий шаг, вне текущего скоупа).
type CommunityNote struct {
	RealIntervalKm int    // что советуют владельцы (напр. 45000); 0 — не задан
	Note           string // человекочитаемый вывод сообщества
	Source         string // URL источника или "demo"
	Reports        int    // сколько отзывов/источников (сила консенсуса); 0 — не задан
}

// Rule — правило регламента (из YAML или LLM). verified=false => демо-данные.
// Category берётся из каталога компонентов (recommender/catalog.go), если не задан в YAML.
// Community — опциональный блок «по отзывам», nil, если данных из отзывов нет.
type Rule struct {
	Code           string
	Title          string
	Operation      string // "replace" | "inspect" | ...
	IntervalKm     int
	IntervalMonths int
	LeadKm         int
	Verified       bool
	Source         string
	Category       string         // "primary" | "secondary"
	Community      *CommunityNote // nil, если данных из отзывов нет
}

// Категории важности компонента (спека — расширение каталога обслуживания).
const (
	CategoryPrimary   = "primary"   // основные (масло, фильтры, тормоза и т.д.)
	CategorySecondary = "secondary" // дополнительные
)

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
	AlertMaintenanceOK              = "MAINTENANCE_OK" // компонент в норме (полный чек-лист)
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
	Category    string         // "primary" | "secondary" (из правила/каталога)
	Community   *CommunityNote // блок «по отзывам»; nil — нет данных
}

// --- B2B ---

// ServiceCenter — b2b-тенант (СТО/дилер), подключённый к своему Bitrix24 по входящему вебхуку.
// BitrixWebhook в памяти — расшифрованный; в БД хранится зашифрованным.
type ServiceCenter struct {
	ID            string
	Name          string
	BitrixWebhook string
	ResponsibleID int // на кого вешать ретеншн-дела в CRM
	CreatedAt     time.Time
}

// ClientCar — авто клиента СТО, прочитанное из его CRM.
type ClientCar struct {
	CRMContactID int64
	ClientName   string
	Make         string
	Model        string
	Year         int
	MileageKm    int
}

// AsVehicle строит Vehicle для рекомендательного слоя (тот же движок, что и в b2c).
func (c ClientCar) AsVehicle() Vehicle {
	return Vehicle{Make: c.Make, Model: c.Model, Year: c.Year, CurrentOdometer: c.MileageKm}
}
