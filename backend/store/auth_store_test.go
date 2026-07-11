package store

import (
	"context"
	"testing"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

func TestAuthStoreRoundtrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	acc, err := s.CreateAccount(ctx)
	if err != nil || acc.ID == "" {
		t.Fatalf("create account: %v", err)
	}
	if err := s.AddIdentity(ctx, domain.Identity{AccountID: acc.ID, Provider: domain.ProviderPassword, ExternalID: "a@x.ru", Secret: "hash"}); err != nil {
		t.Fatal(err)
	}

	// найти existing / отсутствующую identity
	id, found, err := s.FindIdentity(ctx, domain.ProviderPassword, "a@x.ru")
	if err != nil || !found || id.AccountID != acc.ID {
		t.Fatalf("find identity: %+v found=%v err=%v", id, found, err)
	}
	if _, found, _ := s.FindIdentity(ctx, domain.ProviderPassword, "nope@x.ru"); found {
		t.Fatal("несуществующая identity не должна находиться")
	}

	// membership b2c идемпотентно
	if _, err := s.AddMembership(ctx, domain.Membership{AccountID: acc.ID, CtxType: domain.TenantB2C}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddMembership(ctx, domain.Membership{AccountID: acc.ID, CtxType: domain.TenantB2C}); err != nil {
		t.Fatalf("повторный b2c membership не должен падать: %v", err)
	}
	if _, err := s.AddMembership(ctx, domain.Membership{AccountID: acc.ID, CtxType: domain.TenantB2B, TenantID: "sc1", Role: domain.RoleOwner}); err != nil {
		t.Fatal(err)
	}
	mm, err := s.Memberships(ctx, acc.ID)
	if err != nil || len(mm) != 2 {
		t.Fatalf("ожидали 2 контекста, got %d (err=%v)", len(mm), err)
	}
	if _, found, _ := s.FindMembership(ctx, acc.ID, domain.TenantB2B, "sc1"); !found {
		t.Fatal("b2b membership должен находиться")
	}

	// refresh: сохранить → найти → отозвать
	if err := s.SaveRefresh(ctx, acc.ID, "hash1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	gotAcc, _, revoked, found, err := s.GetRefresh(ctx, "hash1")
	if err != nil || !found || revoked || gotAcc != acc.ID {
		t.Fatalf("get refresh: acc=%s revoked=%v found=%v err=%v", gotAcc, revoked, found, err)
	}
	if err := s.RevokeRefresh(ctx, "hash1"); err != nil {
		t.Fatal(err)
	}
	if _, _, revoked, _, _ := s.GetRefresh(ctx, "hash1"); !revoked {
		t.Fatal("после Revoke refresh должен быть revoked")
	}
}
