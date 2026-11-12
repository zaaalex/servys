package auth

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
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
func (m *memStore) SetMembershipRole(_ context.Context, accountID, role string) error {
	for i, x := range m.memberships {
		if x.AccountID == accountID {
			x.Role = role
			m.memberships[i] = x
		}
	}
	return nil
}
func (m *memStore) SaveIdentityProfile(_ context.Context, provider, externalID, displayName, username string) error {
	key := provider + "|" + externalID
	id, ok := m.identities[key]
	if !ok {
		return nil
	}
	if displayName != "" {
		id.DisplayName = displayName
	}
	if username != "" {
		id.Username = username
	}
	m.identities[key] = id
	return nil
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

func TestTelegramLoginIsStable(t *testing.T) {
	svc, _ := newSvc()
	svc.BotToken = "bot"
	ctx := context.Background()
	first, err := svc.LoginTelegram(ctx, telegramInitData("bot", 42))
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.LoginTelegram(ctx, telegramInitData("bot", 42))
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.Verify(first.Access)
	if err != nil || a.CtxType != "b2c" {
		t.Fatalf("verify: %+v err=%v", a, err)
	}
	b, err := svc.Verify(second.Access)
	if err != nil || b.Sub != a.Sub {
		t.Fatalf("повторный вход должен попасть в тот же аккаунт: %+v %+v err=%v", a, b, err)
	}
}

func TestTelegramAdminKeepsRole(t *testing.T) {
	svc, st := newSvc()
	svc.BotToken = "bot"
	svc.AdminID = "42"
	ctx := context.Background()
	tok, err := svc.LoginTelegram(ctx, telegramInitData("bot", 42))
	if err != nil {
		t.Fatal(err)
	}
	claims, err := svc.Verify(tok.Access)
	if err != nil || claims.Role != RoleAdmin {
		t.Fatalf("админ: %+v err=%v", claims, err)
	}
	other, err := svc.LoginTelegram(ctx, telegramInitData("bot", 7))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := svc.Verify(other.Access)
	if err != nil || plain.Role != "" || plain.Sub == claims.Sub {
		t.Fatalf("чужой аккаунт: %+v admin=%s", plain, claims.Sub)
	}
	id, ok, err := st.FindIdentity(ctx, domain.ProviderTelegram, "42")
	if err != nil || !ok || id.AccountID != claims.Sub {
		t.Fatalf("identity: %+v ok=%v err=%v", id, ok, err)
	}
}

func TestRefreshRotates(t *testing.T) {
	svc, _ := newSvc()
	svc.BotToken = "bot"
	ctx := context.Background()
	tok, err := svc.LoginTelegram(ctx, telegramInitData("bot", 7))
	if err != nil {
		t.Fatal(err)
	}
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

func telegramInitData(bot string, userID int64) string {
	user := fmt.Sprintf(`{"id":%d}`, userID)
	pairs := []string{"auth_date=1700000000", "user=" + user}
	sort.Strings(pairs)
	secret := hmac256([]byte("WebAppData"), []byte(bot))
	hash := hex.EncodeToString(hmac256(secret, []byte(strings.Join(pairs, "\n"))))
	q := url.Values{}
	q.Set("auth_date", "1700000000")
	q.Set("user", user)
	q.Set("hash", hash)
	return q.Encode()
}

func TestTelegramDisabledWithoutBotToken(t *testing.T) {
	svc, _ := newSvc() // BotToken пуст
	if _, err := svc.LoginTelegram(context.Background(), "x"); err != ErrProviderDisabled {
		t.Fatalf("без токена бота Telegram-вход выключен: %v", err)
	}
}
