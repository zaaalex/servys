package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

func (s *Store) CreateAccount(ctx context.Context) (domain.Account, error) {
	a := domain.Account{ID: newID(), CreatedAt: time.Now().UTC()}
	_, err := s.db.ExecContext(ctx, `INSERT INTO accounts(id, created_at) VALUES(?,?)`, a.ID, a.CreatedAt)
	return a, err
}

func (s *Store) AddIdentity(ctx context.Context, id domain.Identity) error {
	if id.ID == "" {
		id.ID = newID()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO identities(id, account_id, provider, external_id, secret) VALUES(?,?,?,?,?)`,
		id.ID, id.AccountID, id.Provider, id.ExternalID, id.Secret)
	return err
}

func (s *Store) FindIdentity(ctx context.Context, provider, externalID string) (domain.Identity, bool, error) {
	var id domain.Identity
	err := s.db.QueryRowContext(ctx,
		`SELECT id, account_id, provider, external_id, secret FROM identities WHERE provider = ? AND external_id = ?`,
		provider, externalID).Scan(&id.ID, &id.AccountID, &id.Provider, &id.ExternalID, &id.Secret)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Identity{}, false, nil
	}
	if err != nil {
		return domain.Identity{}, false, err
	}
	return id, true, nil
}

func (s *Store) AddMembership(ctx context.Context, m domain.Membership) (domain.Membership, error) {
	// идемпотентно: если контекст уже есть — вернём его.
	if existing, found, err := s.FindMembership(ctx, m.AccountID, m.CtxType, m.TenantID); err != nil {
		return domain.Membership{}, err
	} else if found {
		return existing, nil
	}
	m.ID = newID()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memberships(id, account_id, ctx_type, tenant_id, role) VALUES(?,?,?,?,?)`,
		m.ID, m.AccountID, string(m.CtxType), m.TenantID, m.Role)
	return m, err
}

func (s *Store) Memberships(ctx context.Context, accountID string) ([]domain.Membership, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, account_id, ctx_type, tenant_id, role FROM memberships WHERE account_id = ? ORDER BY ctx_type`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Membership
	for rows.Next() {
		m, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) FindMembership(ctx context.Context, accountID string, ctxType domain.TenantType, tenantID string) (domain.Membership, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, account_id, ctx_type, tenant_id, role FROM memberships WHERE account_id = ? AND ctx_type = ? AND tenant_id = ?`,
		accountID, string(ctxType), tenantID)
	m, err := scanMembership(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Membership{}, false, nil
	}
	if err != nil {
		return domain.Membership{}, false, err
	}
	return m, true, nil
}

func scanMembership(sc scanner) (domain.Membership, error) {
	var m domain.Membership
	var ct string
	if err := sc.Scan(&m.ID, &m.AccountID, &ct, &m.TenantID, &m.Role); err != nil {
		return domain.Membership{}, err
	}
	m.CtxType = domain.TenantType(ct)
	return m, nil
}

func (s *Store) SaveRefresh(ctx context.Context, accountID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens(token_hash, account_id, expires_at, revoked, created_at) VALUES(?,?,?,0,?)`,
		tokenHash, accountID, expiresAt, time.Now().UTC())
	return err
}

func (s *Store) GetRefresh(ctx context.Context, tokenHash string) (accountID string, expiresAt time.Time, revoked, found bool, err error) {
	var rev int
	err = s.db.QueryRowContext(ctx,
		`SELECT account_id, expires_at, revoked FROM refresh_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&accountID, &expiresAt, &rev)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, false, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, false, err
	}
	return accountID, expiresAt, rev != 0, true, nil
}

func (s *Store) RevokeRefresh(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = 1 WHERE token_hash = ?`, tokenHash)
	return err
}

// ServiceCentersForAccount — СТО, где аккаунт состоит (b2b-membership). Для листинга «своих» в API.
func (s *Store) ServiceCentersForAccount(ctx context.Context, accountID string) ([]domain.ServiceCenter, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sc.id, sc.name, sc.responsible_id, sc.created_at
		FROM service_centers sc
		JOIN memberships m ON m.tenant_id = sc.id AND m.ctx_type = 'b2b'
		WHERE m.account_id = ?
		ORDER BY sc.created_at DESC`, accountID)
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
