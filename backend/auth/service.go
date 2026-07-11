package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/zaaalex/servys/backend/domain"
)

var (
	ErrEmailTaken       = errors.New("auth: email уже занят")
	ErrBadCredentials   = errors.New("auth: неверный email или пароль")
	ErrInvalidRefresh   = errors.New("auth: невалидный refresh-токен")
	ErrNoMembership     = errors.New("auth: нет доступа к этому контексту")
	ErrProviderDisabled = errors.New("auth: провайдер входа выключен")
	ErrValidation       = errors.New("auth: некорректные данные")
)

// Store — хранилище auth (реализует пакет store). Найдено/не найдено — через bool, без sentinel-ошибок.
type Store interface {
	CreateAccount(ctx context.Context) (domain.Account, error)
	AddIdentity(ctx context.Context, id domain.Identity) error
	FindIdentity(ctx context.Context, provider, externalID string) (domain.Identity, bool, error)
	AddMembership(ctx context.Context, m domain.Membership) (domain.Membership, error)
	Memberships(ctx context.Context, accountID string) ([]domain.Membership, error)
	FindMembership(ctx context.Context, accountID string, ctxType domain.TenantType, tenantID string) (domain.Membership, bool, error)
	SaveRefresh(ctx context.Context, accountID, tokenHash string, expiresAt time.Time) error
	GetRefresh(ctx context.Context, tokenHash string) (accountID string, expiresAt time.Time, revoked, found bool, err error)
	RevokeRefresh(ctx context.Context, tokenHash string) error
}

type Service struct {
	Store      Store
	Secret     []byte
	BotToken   string // Telegram; "" => LoginTelegram выключен
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	now        func() time.Time // для тестов; nil => time.Now
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

// Tokens — пара для клиента.
type Tokens struct {
	Access    string `json:"access_token"`
	Refresh   string `json:"refresh_token"`
	ExpiresIn int64  `json:"expires_in"`
}

// Register создаёт аккаунт по email+паролю и личный b2c-контекст.
func (s *Service) Register(ctx context.Context, email, password string) (Tokens, error) {
	if email == "" || len(password) < 6 {
		return Tokens{}, ErrValidation
	}
	_, found, err := s.Store.FindIdentity(ctx, domain.ProviderPassword, email)
	if err != nil {
		return Tokens{}, err
	}
	if found {
		return Tokens{}, ErrEmailTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Tokens{}, err
	}
	acc, err := s.Store.CreateAccount(ctx)
	if err != nil {
		return Tokens{}, err
	}
	if err := s.Store.AddIdentity(ctx, domain.Identity{AccountID: acc.ID, Provider: domain.ProviderPassword, ExternalID: email, Secret: string(hash)}); err != nil {
		return Tokens{}, err
	}
	if _, err := s.Store.AddMembership(ctx, domain.Membership{AccountID: acc.ID, CtxType: domain.TenantB2C}); err != nil {
		return Tokens{}, err
	}
	return s.issue(ctx, acc.ID, domain.TenantB2C, "", "")
}

// Login — вход по email+паролю.
func (s *Service) Login(ctx context.Context, email, password string) (Tokens, error) {
	id, found, err := s.Store.FindIdentity(ctx, domain.ProviderPassword, email)
	if err != nil {
		return Tokens{}, err
	}
	if !found || bcrypt.CompareHashAndPassword([]byte(id.Secret), []byte(password)) != nil {
		return Tokens{}, ErrBadCredentials
	}
	return s.issue(ctx, id.AccountID, domain.TenantB2C, "", "")
}

// LoginTelegram — вход через Telegram Mini App (initData). Заводит аккаунт при первом входе.
func (s *Service) LoginTelegram(ctx context.Context, initData string) (Tokens, error) {
	if s.BotToken == "" {
		return Tokens{}, ErrProviderDisabled
	}
	tgID, err := validateTelegram(initData, s.BotToken)
	if err != nil {
		return Tokens{}, err
	}
	id, found, err := s.Store.FindIdentity(ctx, domain.ProviderTelegram, tgID)
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
	if err := s.Store.AddIdentity(ctx, domain.Identity{AccountID: acc.ID, Provider: domain.ProviderTelegram, ExternalID: tgID}); err != nil {
		return Tokens{}, err
	}
	if _, err := s.Store.AddMembership(ctx, domain.Membership{AccountID: acc.ID, CtxType: domain.TenantB2C}); err != nil {
		return Tokens{}, err
	}
	return s.issue(ctx, acc.ID, domain.TenantB2C, "", "")
}

// Refresh обменивает refresh на новую пару (с ротацией старого). Контекст сбрасывается в b2c.
func (s *Service) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	h := hashToken(refresh)
	accID, exp, revoked, found, err := s.Store.GetRefresh(ctx, h)
	if err != nil {
		return Tokens{}, err
	}
	if !found || revoked || s.clock().After(exp) {
		return Tokens{}, ErrInvalidRefresh
	}
	_ = s.Store.RevokeRefresh(ctx, h) // ротация
	return s.issue(ctx, accID, domain.TenantB2C, "", "")
}

// Logout отзывает refresh.
func (s *Service) Logout(ctx context.Context, refresh string) error {
	return s.Store.RevokeRefresh(ctx, hashToken(refresh))
}

// Memberships — контексты аккаунта (для /auth/me и переключателя).
func (s *Service) Memberships(ctx context.Context, accountID string) ([]domain.Membership, error) {
	return s.Store.Memberships(ctx, accountID)
}

// Switch выдаёт НОВЫЙ access с другим контекстом (refresh не трогаем). Проверяет membership.
func (s *Service) Switch(ctx context.Context, accountID string, ctxType domain.TenantType, tenantID string) (string, error) {
	m, found, err := s.Store.FindMembership(ctx, accountID, ctxType, tenantID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrNoMembership
	}
	return s.access(accountID, ctxType, tenantID, m.Role), nil
}

// Verify проверяет access-токен (для middleware).
func (s *Service) Verify(access string) (Claims, error) {
	return parseJWT(access, s.Secret, s.clock())
}

// --- helpers ---

func (s *Service) access(accountID string, ctxType domain.TenantType, tenant, role string) string {
	now := s.clock()
	return signJWT(Claims{
		Sub: accountID, CtxType: string(ctxType), Tenant: tenant, Role: role,
		Iat: now.Unix(), Exp: now.Add(s.AccessTTL).Unix(),
	}, s.Secret)
}

func (s *Service) issue(ctx context.Context, accountID string, ctxType domain.TenantType, tenant, role string) (Tokens, error) {
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
