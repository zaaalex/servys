package domain

import "time"

const (
	ProviderPassword = "password"
	ProviderTelegram = "telegram"
	ProviderMax      = "max"
	ProviderDev      = "dev"
)

type Account struct {
	ID        string
	CreatedAt time.Time
}

type Identity struct {
	ID          string
	AccountID   string
	Provider    string
	ExternalID  string
	Secret      string
	DisplayName string
	Username    string
}

type Membership struct {
	ID        string
	AccountID string
	CtxType   TenantType
	TenantID  string
	Role      string
}
