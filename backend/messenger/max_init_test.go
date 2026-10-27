package messenger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValidateMaxInitDataDocumentedVector(t *testing.T) {
	const token = "test-max-token"
	const documented = "auth_date=1771409719\n" +
		`chat={"id":12345,"type":"DIALOG"}` + "\n" +
		"ip=192.168.0.1\n" +
		"query_id=4c0ab423-342b-4e45-aea4-2747dbc500cd\n" +
		`user={"id":67890,"first_name":"Max","last_name":"User","username":null,"language_code":"ru","photo_url":null}`
	raw := "chat=%7B%22id%22%3A12345%2C%22type%22%3A%22DIALOG%22%7D" +
		"&ip=192.168.0.1" +
		"&user=%7B%22id%22%3A67890%2C%22first_name%22%3A%22Max%22%2C%22last_name%22%3A%22User%22%2C%22username%22%3Anull%2C%22language_code%22%3A%22ru%22%2C%22photo_url%22%3Anull%7D" +
		"&query_id=4c0ab423-342b-4e45-aea4-2747dbc500cd" +
		"&auth_date=1771409719"
	initData := raw + "&hash=" + hexHMAC(token, documented)
	now := time.Unix(1771409719, 0).Add(time.Minute)

	id, err := ValidateMaxInitData(initData, token, now)
	if err != nil || id != "67890" {
		t.Fatalf("documented vector: id=%q err=%v", id, err)
	}

	upper := raw + "&hash=" + strings.ToUpper(hexHMAC(token, documented))
	id, err = ValidateMaxInitData(upper, token, now)
	if err != nil || id != "67890" {
		t.Fatalf("upper-case hash: id=%q err=%v", id, err)
	}
}

func TestValidateMaxInitData(t *testing.T) {
	const token = "bot-token-ZZZ999"
	now := time.Unix(1_700_000_000, 0)
	user := `{"id":900500,"first_name":"Уникальноеимя"}`
	valid := signedInit(token, map[string]string{
		"auth_date": "1700000000",
		"user":      user,
	})

	tests := []struct {
		name    string
		token   string
		init    string
		now     time.Time
		wantID  string
		wantErr error
		reason  string
		hidden  string
	}{
		{name: "ok", token: token, init: valid, now: now, wantID: "900500"},
		{name: "not configured", token: "", init: valid, now: now, wantErr: ErrNotConfigured},
		{name: "empty", token: token, init: "  \n", now: now, wantErr: ErrInvalidInitData, reason: "empty"},
		{name: "hash missing", token: token, init: "auth_date=1&user=%7B%7D", now: now, wantErr: ErrInvalidInitData, reason: "hash missing"},
		{name: "empty hash", token: token, init: "auth_date=1&hash=", now: now, wantErr: ErrInvalidInitData, reason: "hash missing"},
		{name: "malformed key", token: token, init: "=value&hash=abc", now: now, wantErr: ErrInvalidInitData, reason: "malformed"},
		{name: "duplicate", token: token, init: valid + "&user=" + url.QueryEscape(`{"id":1}`), now: now, wantErr: ErrInvalidInitData, reason: "duplicate key"},
		{name: "duplicate decoded key", token: token, init: signedInit(token, map[string]string{"auth_date": "1700000000", "user": user}) + "&%75ser=x", now: now, wantErr: ErrInvalidInitData, reason: "duplicate key"},
		{
			name: "wrong token does not inspect user", token: "another-token",
			init: signedInit("bot-token-ZZZ999", map[string]string{"auth_date": "1", "user": user}),
			now:  now, wantErr: ErrInvalidInitData, reason: "hash mismatch", hidden: "Уникальноеимя",
		},
		{name: "skip stray pairs", token: token, init: valid + "&&flag&", now: now, wantID: "900500"},
		{name: "plus is space", token: token, init: plusSpaceInit(token, now), now: now, wantID: "7"},
		{name: "exact day", token: token, init: signedAt(token, now.Add(-24*time.Hour), "5"), now: now, wantID: "5"},
		{name: "older than day", token: token, init: signedAt(token, now.Add(-24*time.Hour-time.Second), "5"), now: now, wantErr: ErrInvalidInitData, reason: "expired"},
		{name: "exact five minutes", token: token, init: signedAt(token, now.Add(5*time.Minute), "5"), now: now, wantID: "5"},
		{name: "from the future", token: token, init: signedAt(token, now.Add(5*time.Minute+time.Second), "5"), now: now, wantErr: ErrInvalidInitData, reason: "from the future"},
		{name: "bad auth date", token: token, init: signedInit(token, map[string]string{"auth_date": "17e5", "user": `{"id":5}`}), now: now, wantErr: ErrInvalidInitData, reason: "auth_date invalid"},
		{name: "missing user", token: token, init: signedInit(token, map[string]string{"auth_date": "1700000000"}), now: now, wantErr: ErrInvalidInitData, reason: "user invalid"},
		{name: "string id", token: token, init: signedInit(token, map[string]string{"auth_date": "1700000000", "user": `{"id":"5"}`}), now: now, wantErr: ErrInvalidInitData, reason: "user invalid"},
		{name: "zero id", token: token, init: signedInit(token, map[string]string{"auth_date": "1700000000", "user": `{"id":0}`}), now: now, wantErr: ErrInvalidInitData, reason: "user invalid"},
		{name: "fraction id", token: token, init: signedInit(token, map[string]string{"auth_date": "1700000000", "user": `{"id":1.5}`}), now: now, wantErr: ErrInvalidInitData, reason: "user invalid"},
		{name: "negative id", token: token, init: signedInit(token, map[string]string{"auth_date": "1700000000", "user": `{"id":-5}`}), now: now, wantID: "-5"},
		{name: "wide id", token: token, init: signedInit(token, map[string]string{"auth_date": "1700000000", "user": `{"id":9007199254740993}`}), now: now, wantID: "9007199254740993"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ValidateMaxInitData(tt.init, tt.token, tt.now)
			if tt.wantID != "" {
				if err != nil || id != tt.wantID {
					t.Fatalf("id=%q err=%v", id, err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err=%v, want %v", err, tt.wantErr)
			}
			if tt.reason != "" && !strings.Contains(err.Error(), tt.reason) {
				t.Fatalf("err=%v, want reason %q", err, tt.reason)
			}
			if tt.token != "" && strings.Contains(err.Error(), tt.token) {
				t.Fatalf("error leaks token: %v", err)
			}
			if tt.hidden != "" && strings.Contains(err.Error(), tt.hidden) {
				t.Fatalf("error leaks init data: %v", err)
			}
		})
	}
}

func signedAt(token string, at time.Time, id string) string {
	return signedInit(token, map[string]string{
		"auth_date": strconv.FormatInt(at.Unix(), 10),
		"user":      `{"id":` + id + `}`,
	})
}

func signedInit(token string, fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var check strings.Builder
	vals := url.Values{}
	for i, key := range keys {
		if i > 0 {
			check.WriteByte('\n')
		}
		check.WriteString(key)
		check.WriteByte('=')
		check.WriteString(fields[key])
		vals.Set(key, fields[key])
	}
	vals.Set("hash", hexHMAC(token, check.String()))
	return vals.Encode()
}

func plusSpaceInit(token string, now time.Time) string {
	fields := map[string]string{
		"auth_date":   strconv.FormatInt(now.Unix(), 10),
		"start_param": "hello world",
		"user":        `{"id":7}`,
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var check strings.Builder
	var raw strings.Builder
	for i, key := range keys {
		if i > 0 {
			check.WriteByte('\n')
			raw.WriteByte('&')
		}
		check.WriteString(key + "=" + fields[key])
		raw.WriteString(key + "=" + strings.ReplaceAll(url.QueryEscape(fields[key]), "%20", "+"))
	}
	raw.WriteString("&hash=" + hexHMAC(token, check.String()))
	return raw.String()
}

func hexHMAC(token, data string) string {
	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMac.Write([]byte(token))
	mac := hmac.New(sha256.New, secretMac.Sum(nil))
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
