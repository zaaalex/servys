package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/messenger"
)

var (
	ErrInvalidRefresh   = errors.New("auth: невалидный refresh-токен")
	ErrProviderDisabled = errors.New("auth: провайдер входа выключен")
	ErrMaxInvalid       = errors.New("auth: невалидные данные MAX")
)

// Store — хранилище auth (реализует пакет store). Найдено/не найдено — через bool, без sentinel-ошибок.
type Store interface {
	CreateAccount(ctx context.Context) (domain.Account, error)
	AddIdentity(ctx context.Context, id domain.Identity) error
	FindIdentity(ctx context.Context, provider, externalID string) (domain.Identity, bool, error)
	AddMembership(ctx context.Context, m domain.Membership) (domain.Membership, error)
	Memberships(ctx context.Context, accountID string) ([]domain.Membership, error)
	SetMembershipRole(ctx context.Context, accountID, role string) error
	SaveIdentityProfile(ctx context.Context, provider, externalID, displayName, username string) error
	SaveRefresh(ctx context.Context, accountID, tokenHash string, expiresAt time.Time) error
	GetRefresh(ctx context.Context, tokenHash string) (accountID string, expiresAt time.Time, revoked, found bool, err error)
	RevokeRefresh(ctx context.Context, tokenHash string) error
	RevokeRefreshOnce(ctx context.Context, tokenHash string) (bool, error)
}

type Service struct {
	Store       Store
	Secret      []byte
	BotToken    string // Telegram; "" => LoginTelegram выключен
	MaxBotToken string // MAX; "" => LoginMax выключен
	DevLogin    bool   // локальный просмотр гаража без мессенджера
	AdminID     string // Telegram id, который при входе становится администратором
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	now         func() time.Time // для тестов; nil => time.Now
}

func New(store Store, secret []byte) *Service {
	return &Service{
		Store:      store,
		Secret:     secret,
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 30 * 24 * time.Hour,
	}
}

func (s *Service) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

type Tokens struct {
	Access    string `json:"access_token"`
	Refresh   string `json:"refresh_token"`
	ExpiresIn int64  `json:"expires_in"`
}

const RoleAdmin = "admin"

func (s *Service) LoginTelegram(ctx context.Context, initData string) (Tokens, error) {
	if s.BotToken == "" {
		return Tokens{}, ErrProviderDisabled
	}
	person, err := validateTelegram(initData, s.BotToken, s.clock())
	if err != nil {
		return Tokens{}, err
	}
	id, found, err := s.Store.FindIdentity(ctx, domain.ProviderTelegram, person.ID)
	if err != nil {
		return Tokens{}, err
	}
	accountID := id.AccountID
	if !found {
		acc, err := s.Store.CreateAccount(ctx)
		if err != nil {
			return Tokens{}, err
		}
		accountID = acc.ID
		if err := s.Store.AddIdentity(ctx, domain.Identity{
			AccountID: acc.ID, Provider: domain.ProviderTelegram, ExternalID: person.ID,
			DisplayName: person.Name, Username: person.Username,
		}); err != nil {
			return Tokens{}, err
		}
		if _, err := s.Store.AddMembership(ctx, domain.Membership{AccountID: acc.ID, CtxType: domain.TenantB2C}); err != nil {
			return Tokens{}, err
		}
	} else if err := s.Store.SaveIdentityProfile(ctx, domain.ProviderTelegram, person.ID, person.Name, person.Username); err != nil {
		return Tokens{}, err
	}
	if s.AdminID != "" && person.ID == s.AdminID {
		if err := s.Store.SetMembershipRole(ctx, accountID, RoleAdmin); err != nil {
			return Tokens{}, err
		}
	}
	return s.issue(ctx, accountID, domain.TenantB2C, "", "")
}

func (s *Service) LoginMax(ctx context.Context, initData string) (Tokens, error) {
	if s.MaxBotToken == "" {
		return Tokens{}, ErrProviderDisabled
	}
	maxID, err := messenger.ValidateMaxInitData(initData, s.MaxBotToken, s.clock())
	if err != nil {
		if errors.Is(err, messenger.ErrNotConfigured) {
			return Tokens{}, ErrProviderDisabled
		}
		return Tokens{}, ErrMaxInvalid
	}
	id, found, err := s.Store.FindIdentity(ctx, domain.ProviderMax, maxID)
	if err != nil {
		return Tokens{}, err
	}
	if found {
		return s.issue(ctx, id.AccountID, domain.TenantB2C, "", "")
	}
	acc, err := s.Store.CreateAccount(ctx)
	if err != nil {
		return Tokens{}, err
	}
	if err := s.Store.AddIdentity(ctx, domain.Identity{AccountID: acc.ID, Provider: domain.ProviderMax, ExternalID: maxID}); err != nil {
		return Tokens{}, err
	}
	if _, err := s.Store.AddMembership(ctx, domain.Membership{AccountID: acc.ID, CtxType: domain.TenantB2C}); err != nil {
		return Tokens{}, err
	}
	return s.issue(ctx, acc.ID, domain.TenantB2C, "", "")
}

// LoginDev открывает один локальный гараж. Выключен, пока DevLogin ложен.
// asAdmin поднимает этот локальный аккаунт до администратора. На сервере маршрут не регистрируется.
func (s *Service) LoginDev(ctx context.Context, asAdmin bool) (Tokens, error) {
	if !s.DevLogin {
		return Tokens{}, ErrProviderDisabled
	}
	id, found, err := s.Store.FindIdentity(ctx, domain.ProviderDev, "local")
	if err != nil {
		return Tokens{}, err
	}
	accountID := id.AccountID
	if !found {
		acc, err := s.Store.CreateAccount(ctx)
		if err != nil {
			return Tokens{}, err
		}
		accountID = acc.ID
		if err := s.Store.AddIdentity(ctx, domain.Identity{AccountID: acc.ID, Provider: domain.ProviderDev, ExternalID: "local"}); err != nil {
			return Tokens{}, err
		}
		if _, err := s.Store.AddMembership(ctx, domain.Membership{AccountID: acc.ID, CtxType: domain.TenantB2C}); err != nil {
			return Tokens{}, err
		}
	}
	if asAdmin {
		if err := s.Store.SetMembershipRole(ctx, accountID, RoleAdmin); err != nil {
			return Tokens{}, err
		}
	}
	return s.issue(ctx, accountID, domain.TenantB2C, "", "")
}

// Новая пара всегда с контекстом личного гаража.
func (s *Service) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	h := hashToken(refresh)
	accID, exp, revoked, found, err := s.Store.GetRefresh(ctx, h)
	if err != nil {
		return Tokens{}, err
	}
	if !found || revoked || s.clock().After(exp) {
		return Tokens{}, ErrInvalidRefresh
	}
	ok, err := s.Store.RevokeRefreshOnce(ctx, h)
	if err != nil {
		return Tokens{}, err
	}
	if !ok {
		return Tokens{}, ErrInvalidRefresh
	}
	return s.issue(ctx, accID, domain.TenantB2C, "", "")
}

func (s *Service) Logout(ctx context.Context, refresh string) error {
	return s.Store.RevokeRefresh(ctx, hashToken(refresh))
}

func (s *Service) Memberships(ctx context.Context, accountID string) ([]domain.Membership, error) {
	return s.Store.Memberships(ctx, accountID)
}

func (s *Service) Verify(access string) (Claims, error) {
	return parseJWT(access, s.Secret, s.clock())
}

func (s *Service) access(accountID string, ctxType domain.TenantType, tenant, role string) string {
	now := s.clock()
	return signJWT(Claims{
		Sub: accountID, CtxType: string(ctxType), Tenant: tenant, Role: role,
		Iat: now.Unix(), Exp: now.Add(s.AccessTTL).Unix(),
	}, s.Secret)
}

func (s *Service) issue(ctx context.Context, accountID string, ctxType domain.TenantType, tenant, role string) (Tokens, error) {
	if role == "" {
		list, err := s.Store.Memberships(ctx, accountID)
		if err != nil {
			return Tokens{}, err
		}
		for _, m := range list {
			if m.Role == RoleAdmin {
				role = RoleAdmin
				break
			}
		}
	}
	refresh := randToken()
	if err := s.Store.SaveRefresh(ctx, accountID, hashToken(refresh), s.clock().Add(s.RefreshTTL)); err != nil {
		return Tokens{}, err
	}
	return Tokens{Access: s.access(accountID, ctxType, tenant, role), Refresh: refresh, ExpiresIn: int64(s.AccessTTL.Seconds())}, nil
}

func randToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}
