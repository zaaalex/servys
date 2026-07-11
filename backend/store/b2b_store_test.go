package store

import (
	"context"
	"errors"
	"testing"

	"github.com/zaaalex/servys/backend/crypto"
	"github.com/zaaalex/servys/backend/domain"
)

func TestServiceCenterCRUDAndDedupe(t *testing.T) {
	s := newTestStore(t)
	c, _ := crypto.New("test-key")
	s.SetCipher(c)
	ctx := context.Background()

	webhook := "https://acme.bitrix24.ru/rest/1/tok/"
	sc, err := s.AddServiceCenter(ctx, domain.ServiceCenter{Name: "СТО-1", BitrixWebhook: webhook, ResponsibleID: 7})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetServiceCenter(ctx, sc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.BitrixWebhook != webhook || got.ResponsibleID != 7 || got.Name != "СТО-1" {
		t.Fatalf("get sc: %+v", got)
	}

	list, err := s.ListServiceCenters(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if list[0].BitrixWebhook != "" {
		t.Fatal("в листинге вебхук не должен возвращаться")
	}

	// dedupe
	if done, _ := s.AlreadyPushed(ctx, sc.ID, "k1"); done {
		t.Fatal("ещё не пушили")
	}
	if err := s.RecordPush(ctx, sc.ID, "k1", "r1"); err != nil {
		t.Fatal(err)
	}
	if done, _ := s.AlreadyPushed(ctx, sc.ID, "k1"); !done {
		t.Fatal("после RecordPush должно быть done")
	}
	// повторная запись того же ключа не падает (идемпотентно)
	if err := s.RecordPush(ctx, sc.ID, "k1", "r1"); err != nil {
		t.Fatalf("повторный RecordPush: %v", err)
	}
}

func TestB2BMethodsRequireCipher(t *testing.T) {
	s := newTestStore(t) // без cipher
	_, err := s.AddServiceCenter(context.Background(), domain.ServiceCenter{Name: "x", BitrixWebhook: "y"})
	if !errors.Is(err, ErrCipherNotSet) {
		t.Fatalf("без cipher ожидали ErrCipherNotSet, got %v", err)
	}
}
