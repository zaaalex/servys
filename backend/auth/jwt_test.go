package auth

import (
	"testing"
	"time"
)

func TestJWTRoundtrip(t *testing.T) {
	secret := []byte("s3cret")
	now := time.Unix(1_000_000, 0)
	c := Claims{Sub: "acc1", CtxType: "b2b", Tenant: "sc1", Role: "manager", Iat: now.Unix(), Exp: now.Add(15 * time.Minute).Unix()}
	tok := signJWT(c, secret)

	got, err := parseJWT(tok, secret, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sub != "acc1" || got.CtxType != "b2b" || got.Tenant != "sc1" || got.Role != "manager" {
		t.Fatalf("claims не совпали: %+v", got)
	}
}

func TestJWTRejectsTamperedSignature(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	tok := signJWT(Claims{Sub: "acc1", Exp: now.Add(time.Hour).Unix()}, []byte("a"))
	if _, err := parseJWT(tok, []byte("b"), now); err != ErrInvalidToken {
		t.Fatalf("подпись чужим секретом должна отвергаться, err=%v", err)
	}
}

func TestJWTRejectsExpired(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_000_000, 0)
	tok := signJWT(Claims{Sub: "acc1", Exp: now.Add(-time.Second).Unix()}, secret)
	if _, err := parseJWT(tok, secret, now); err != ErrExpired {
		t.Fatalf("истёкший токен должен отвергаться, err=%v", err)
	}
}
