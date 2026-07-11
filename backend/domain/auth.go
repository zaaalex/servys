package domain

import "time"

// Провайдеры входа (точки входа). bitrix — слот на будущее; в MVP это интеграция, не вход.
const (
	ProviderPassword = "password" // email + пароль (веб/мобилка)
	ProviderTelegram = "telegram" // Telegram Mini App / Login
	ProviderBitrix   = "bitrix"   // задел: вход из Bitrix (пока не используется)
)

// Роли в b2b-тенанте (СТО).
const (
	RoleOwner   = "owner"
	RoleManager = "manager"
	RoleMember  = "member"
)

// Account — единый аккаунт пользователя. Один на человека независимо от точки входа.
type Account struct {
	ID        string
	CreatedAt time.Time
}

// Identity — способ входа, привязанный к аккаунту. Один аккаунт → много identity.
type Identity struct {
	ID         string
	AccountID  string
	Provider   string // ProviderPassword | ProviderTelegram | ...
	ExternalID string // email | telegram user id | ...
	Secret     string // bcrypt-хэш для password; иначе пусто
}

// Membership — контекст, в котором аккаунт может действовать.
//   b2c: CtxType=b2c, TenantID="" (личный гараж, есть всегда)
//   b2b: CtxType=b2b, TenantID=<id СТО>, Role=<роль>
type Membership struct {
	ID        string
	AccountID string
	CtxType   TenantType
	TenantID  string
	Role      string
}
