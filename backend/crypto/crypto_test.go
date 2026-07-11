package crypto

import "testing"

func TestSealOpenRoundtrip(t *testing.T) {
	c, err := New("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	plain := "https://acme.bitrix24.ru/rest/1/SECRET/"
	enc, err := c.Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if enc == plain {
		t.Fatal("шифротекст равен открытому тексту")
	}
	got, err := c.Open(enc)
	if err != nil || got != plain {
		t.Fatalf("Open: got=%q err=%v", got, err)
	}
}

func TestOpenWithWrongKeyFails(t *testing.T) {
	a, _ := New("key-a")
	b, _ := New("key-b")
	enc, _ := a.Seal("secret")
	if _, err := b.Open(enc); err == nil {
		t.Fatal("расшифровка чужим ключом должна падать")
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("пустой ключ должен отвергаться")
	}
}
