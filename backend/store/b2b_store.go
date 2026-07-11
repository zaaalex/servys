package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

// ErrCipherNotSet — b2b-методы вызваны без ключа шифрования (APP_SECRET_KEY).
var ErrCipherNotSet = errors.New("b2b: cipher not set (APP_SECRET_KEY required)")

// AddServiceCenter сохраняет СТО, шифруя вебхук.
func (s *Store) AddServiceCenter(ctx context.Context, sc domain.ServiceCenter) (domain.ServiceCenter, error) {
	if s.cipher == nil {
		return domain.ServiceCenter{}, ErrCipherNotSet
	}
	enc, err := s.cipher.Seal(sc.BitrixWebhook)
	if err != nil {
		return domain.ServiceCenter{}, err
	}
	sc.ID = newID()
	sc.CreatedAt = time.Now().UTC()
	if sc.ResponsibleID <= 0 {
		sc.ResponsibleID = 1
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO service_centers(id, name, webhook_enc, responsible_id, created_at) VALUES(?,?,?,?,?)`,
		sc.ID, sc.Name, enc, sc.ResponsibleID, sc.CreatedAt); err != nil {
		return domain.ServiceCenter{}, err
	}
	return sc, nil
}

func (s *Store) GetServiceCenter(ctx context.Context, id string) (domain.ServiceCenter, error) {
	if s.cipher == nil {
		return domain.ServiceCenter{}, ErrCipherNotSet
	}
	var sc domain.ServiceCenter
	var enc string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, webhook_enc, responsible_id, created_at FROM service_centers WHERE id = ?`, id).
		Scan(&sc.ID, &sc.Name, &enc, &sc.ResponsibleID, &sc.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ServiceCenter{}, ErrNotFound
	}
	if err != nil {
		return domain.ServiceCenter{}, err
	}
	if sc.BitrixWebhook, err = s.cipher.Open(enc); err != nil {
		return domain.ServiceCenter{}, err
	}
	return sc, nil
}

// ListServiceCenters возвращает СТО (без расшифровки вебхука — для листинга он не нужен).
func (s *Store) ListServiceCenters(ctx context.Context) ([]domain.ServiceCenter, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, responsible_id, created_at FROM service_centers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ServiceCenter
	for rows.Next() {
		var sc domain.ServiceCenter
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.ResponsibleID, &sc.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// AlreadyPushed сообщает, создавали ли мы уже действие с таким dedupe_key для тенанта.
func (s *Store) AlreadyPushed(ctx context.Context, tenantID, dedupeKey string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM pushed_actions WHERE tenant_id = ? AND dedupe_key = ?`, tenantID, dedupeKey).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// RecordPush фиксирует созданное действие (идемпотентно — UNIQUE(tenant, key)).
func (s *Store) RecordPush(ctx context.Context, tenantID, dedupeKey, remoteID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO pushed_actions(id, tenant_id, dedupe_key, remote_id, created_at) VALUES(?,?,?,?,?)`,
		newID(), tenantID, dedupeKey, remoteID, time.Now().UTC())
	return err
}
