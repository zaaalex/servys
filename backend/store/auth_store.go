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
		`INSERT INTO identities(id, account_id, provider, external_id, secret, display_name, username) VALUES(?,?,?,?,?,?,?)`,
		id.ID, id.AccountID, id.Provider, id.ExternalID, id.Secret, id.DisplayName, id.Username)
	return err
}

func (s *Store) FindIdentity(ctx context.Context, provider, externalID string) (domain.Identity, bool, error) {
	var id domain.Identity
	err := s.db.QueryRowContext(ctx,
		`SELECT id, account_id, provider, external_id, secret, display_name, username FROM identities WHERE provider = ? AND external_id = ?`,
		provider, externalID).Scan(&id.ID, &id.AccountID, &id.Provider, &id.ExternalID, &id.Secret, &id.DisplayName, &id.Username)
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

func (s *Store) SetMembershipRole(ctx context.Context, accountID, role string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE memberships SET role = ? WHERE account_id = ? AND ctx_type = ?`,
		role, accountID, string(domain.TenantB2C))
	return err
}

func (s *Store) SaveIdentityProfile(ctx context.Context, provider, externalID, displayName, username string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE identities
		SET display_name = CASE WHEN ? != '' THEN ? ELSE display_name END,
		    username = CASE WHEN ? != '' THEN ? ELSE username END
		WHERE provider = ? AND external_id = ?`,
		displayName, displayName, username, username, provider, externalID)
	return err
}

// TelegramAccount — человек, который входит из Telegram под своим аккаунтом.
type TelegramAccount struct {
	AccountID  string
	TelegramID string
	Name       string
	Username   string
	Role       string
	CreatedAt  string
}

func (s *Store) ListTelegramAccounts(ctx context.Context) ([]TelegramAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, i.external_id, i.display_name, i.username, COALESCE(m.role, ''), a.created_at
		FROM identities i
		JOIN accounts a ON a.id = i.account_id
		LEFT JOIN memberships m ON m.account_id = a.id AND m.ctx_type = ?
		WHERE i.provider = ?
		ORDER BY a.created_at, a.id`, string(domain.TenantB2C), domain.ProviderTelegram)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TelegramAccount
	for rows.Next() {
		var p TelegramAccount
		if err := rows.Scan(&p.AccountID, &p.TelegramID, &p.Name, &p.Username, &p.Role, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// EnsureTelegramAccount заводит аккаунт для номера Telegram, если его ещё нет.
// Повторный вызов возвращает тот же аккаунт.
func (s *Store) EnsureTelegramAccount(ctx context.Context, telegramID, name string) (TelegramAccount, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TelegramAccount{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var existing string
	err = tx.QueryRowContext(ctx,
		`SELECT account_id FROM identities WHERE provider = ? AND external_id = ?`,
		domain.ProviderTelegram, telegramID).Scan(&existing)
	created := false
	if errors.Is(err, sql.ErrNoRows) {
		acc := domain.Account{ID: newID(), CreatedAt: time.Now().UTC()}
		if _, err := tx.ExecContext(ctx, `INSERT INTO accounts(id, created_at) VALUES(?,?)`, acc.ID, acc.CreatedAt); err != nil {
			return TelegramAccount{}, false, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO identities(id, account_id, provider, external_id, secret, display_name, username) VALUES(?,?,?,?,?,?,?)`,
			newID(), acc.ID, domain.ProviderTelegram, telegramID, "", name, ""); err != nil {
			return TelegramAccount{}, false, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO memberships(id, account_id, ctx_type, tenant_id, role) VALUES(?,?,?,?,?)`,
			newID(), acc.ID, string(domain.TenantB2C), "", ""); err != nil {
			return TelegramAccount{}, false, err
		}
		created = true
	} else if err != nil {
		return TelegramAccount{}, false, err
	} else if name != "" {
		if _, err := tx.ExecContext(ctx,
			`UPDATE identities SET display_name = ? WHERE provider = ? AND external_id = ? AND display_name = ''`,
			name, domain.ProviderTelegram, telegramID); err != nil {
			return TelegramAccount{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return TelegramAccount{}, false, err
	}
	person, found, err := s.telegramAccount(ctx, telegramID)
	if err != nil || !found {
		if err == nil {
			err = sql.ErrNoRows
		}
		return TelegramAccount{}, false, err
	}
	return person, created, nil
}

func (s *Store) telegramAccount(ctx context.Context, telegramID string) (TelegramAccount, bool, error) {
	var p TelegramAccount
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, i.external_id, i.display_name, i.username, COALESCE(m.role, ''), a.created_at
		FROM identities i
		JOIN accounts a ON a.id = i.account_id
		LEFT JOIN memberships m ON m.account_id = a.id AND m.ctx_type = ?
		WHERE i.provider = ? AND i.external_id = ?`,
		string(domain.TenantB2C), domain.ProviderTelegram, telegramID).
		Scan(&p.AccountID, &p.TelegramID, &p.Name, &p.Username, &p.Role, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TelegramAccount{}, false, nil
	}
	if err != nil {
		return TelegramAccount{}, false, err
	}
	return p, true, nil
}

func (s *Store) TelegramAccountByID(ctx context.Context, accountID string) (TelegramAccount, bool, error) {
	var p TelegramAccount
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, i.external_id, i.display_name, i.username, COALESCE(m.role, ''), a.created_at
		FROM identities i
		JOIN accounts a ON a.id = i.account_id
		LEFT JOIN memberships m ON m.account_id = a.id AND m.ctx_type = ?
		WHERE i.provider = ? AND i.account_id = ?`,
		string(domain.TenantB2C), domain.ProviderTelegram, accountID).
		Scan(&p.AccountID, &p.TelegramID, &p.Name, &p.Username, &p.Role, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TelegramAccount{}, false, nil
	}
	if err != nil {
		return TelegramAccount{}, false, err
	}
	return p, true, nil
}

func (s *Store) ensureIdentityColumns() error {
	rows, err := s.db.Query(`PRAGMA table_info(identities)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	haveName, haveUser := false, false
	for rows.Next() {
		var cid, notNull, pk int
		var name string
		var typ, def sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
			return err
		}
		switch name {
		case "display_name":
			haveName = true
		case "username":
			haveUser = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !haveName {
		if _, err := s.db.Exec(`ALTER TABLE identities ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !haveUser {
		if _, err := s.db.Exec(`ALTER TABLE identities ADD COLUMN username TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
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
