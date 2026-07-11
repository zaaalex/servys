package auth

import (
	"context"
	"testing"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

// memStore — in-memory реализация auth.Store для тестов.
type memStore struct {
	seq         int
	accounts    map[string]bool
	identities  map[string]domain.Identity // key: provider|external
	memberships []domain.Membership
	refresh     map[string]refreshRec
}

type refreshRec struct {
	accountID string
	exp       time.Time
	revoked   bool
}

func newMemStore() *memStore {
	return &memStore{accounts: map[string]bool{}, identities: map[string]domain.Identity{}, refresh: map[string]refreshRec{}}
}

func (m *memStore) id(prefix string) string { m.seq++; return prefix + string(rune('a'+m.seq)) }

func (m *memStore) CreateAccount(context.Context) (domain.Account, error) {
	a := domain.Account{ID: m.id("acc-")}
	m.accounts[a.ID] = true
	return a, nil
}
func (m *memStore) AddIdentity(_ context.Context, id domain.Identity) error {
	m.identities[id.Provider+"|"+id.ExternalID] = id
	return nil
}
func (m *memStore) FindIdentity(_ context.Context, provider, ext string) (domain.Identity, bool, error) {
	id, ok := m.identities[provider+"|"+ext]
	return id, ok, nil
}
func (m *memStore) AddMembership(_ context.Context, mm domain.Membership) (domain.Membership, error) {
	mm.ID = m.id("mem-")
	m.memberships = append(m.memberships, mm)
	return mm, nil
}
func (m *memStore) Memberships(_ context.Context, accountID string) ([]domain.Membership, error) {
	var out []domain.Membership
	for _, x := range m.memberships {
		if x.AccountID == accountID {
			out = append(out, x)
		}
	}
	return out, nil
}
func (m *memStore) FindMembership(_ context.Context, accountID string, ctxType domain.TenantType, tenantID string) (domain.Membership, bool, error) {
	for _, x := range m.memberships {
		if x.AccountID == accountID && x.CtxType == ctxType && x.TenantID == tenantID {
			return x, true, nil
		}
	}
	return domain.Membership{}, false, nil
}
func (m *memStore) SaveRefresh(_ context.Context, accountID, hash string, exp time.Time) error {
	m.refresh[hash] = refreshRec{accountID: accountID, exp: exp}
	return nil
}
func (m *memStore) GetRefresh(_ context.Context, hash string) (string, time.Time, bool, bool, error) {
	r, ok := m.refresh[hash]
	return r.accountID, r.exp, r.revoked, ok, nil
}
func (m *memStore) RevokeRefresh(_ context.Context, hash string) error {
	if r, ok := m.refresh[hash]; ok {
		r.revoked = true
		m.refresh[hash] = r
	}
	return nil
}

func newSvc() (*Service, *memStore) {
	st := newMemStore()
	return New(st, []byte("test-secret")), st
}

func TestRegisterThenLogin(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	if _, err := svc.Register(ctx, "ivan@x.ru", "secret1"); err != nil {
		t.Fatal(err)
	}
	tok, err := svc.Login(ctx, "ivan@x.ru", "secret1")
	if err != nil {
		t.Fatal(err)
	}
	c, err := svc.Verify(tok.Access)
	if err != nil || c.CtxType != "b2c" {
		t.Fatalf("verify: %+v err=%v", c, err)
	}
}

func TestRegisterDuplicateAndBadLogin(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	_, _ = svc.Register(ctx, "a@x.ru", "secret1")
	if _, err := svc.Register(ctx, "a@x.ru", "secret2"); err != ErrEmailTaken {
		t.Fatalf("дубль email: %v", err)
	}
	if _, err := svc.Login(ctx, "a@x.ru", "wrong"); err != ErrBadCredentials {
		t.Fatalf("неверный пароль: %v", err)
	}
	if _, err := svc.Login(ctx, "nope@x.ru", "secret1"); err != ErrBadCredentials {
		t.Fatalf("нет юзера: %v", err)
	}
}

func TestRefreshRotates(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	tok, _ := svc.Register(ctx, "a@x.ru", "secret1")
	tok2, err := svc.Refresh(ctx, tok.Refresh)
	if err != nil {
		t.Fatal(err)
	}
	if tok2.Refresh == tok.Refresh {
		t.Fatal("refresh должен ротироваться")
	}
	// старый refresh больше не работает
	if _, err := svc.Refresh(ctx, tok.Refresh); err != ErrInvalidRefresh {
		t.Fatalf("старый refresh должен быть отозван: %v", err)
	}
}

func TestSwitchRequiresMembership(t *testing.T) {
	svc, st := newSvc()
	ctx := context.Background()
	tok, _ := svc.Register(ctx, "a@x.ru", "secret1")
	c, _ := svc.Verify(tok.Access)
	acc := c.Sub

	// нет b2b-членства → отказ
	if _, err := svc.Switch(ctx, acc, domain.TenantB2B, "sc1"); err != ErrNoMembership {
		t.Fatalf("без membership ожидали отказ: %v", err)
	}
	// добавили членство в СТО → переключение выдаёт access с этим контекстом
	_, _ = st.AddMembership(ctx, domain.Membership{AccountID: acc, CtxType: domain.TenantB2B, TenantID: "sc1", Role: domain.RoleManager})
	access, err := svc.Switch(ctx, acc, domain.TenantB2B, "sc1")
	if err != nil {
		t.Fatal(err)
	}
	claims, _ := svc.Verify(access)
	if claims.CtxType != "b2b" || claims.Tenant != "sc1" || claims.Role != domain.RoleManager {
		t.Fatalf("контекст в токене неверный: %+v", claims)
	}
}

func TestTelegramDisabledWithoutBotToken(t *testing.T) {
	svc, _ := newSvc() // BotToken пуст
	if _, err := svc.LoginTelegram(context.Background(), "x"); err != ErrProviderDisabled {
		t.Fatalf("без токена бота Telegram-вход выключен: %v", err)
	}
}
