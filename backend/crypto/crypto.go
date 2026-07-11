// Package crypto — шифрование секретов (вебхуков СТО) для хранения в БД.
// AES-256-GCM, ключ из APP_SECRET_KEY (любой длины → sha256). ADR-001 §10.4.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

type Cipher struct{ gcm cipher.AEAD }

// New создаёт шифратор из строкового ключа (нормализуется через sha256 до 32 байт).
func New(key string) (*Cipher, error) {
	if key == "" {
		return nil, errors.New("crypto: пустой ключ")
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{gcm: gcm}, nil
}

// FromEnv берёт ключ из APP_SECRET_KEY. Пустой ключ => ошибка (b2b без ключа не поднимается).
func FromEnv() (*Cipher, error) { return New(os.Getenv("APP_SECRET_KEY")) }

// Seal шифрует строку → base64(nonce|ciphertext).
func (c *Cipher) Seal(plain string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := c.gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Open расшифровывает то, что вернул Seal.
func (c *Cipher) Open(enc string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	ns := c.gcm.NonceSize()
	if len(data) < ns {
		return "", errors.New("crypto: слишком короткий шифротекст")
	}
	nonce, ct := data[:ns], data[ns:]
	pt, err := c.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
