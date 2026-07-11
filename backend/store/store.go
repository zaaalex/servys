// Package store — SQLite-хранилище (Dev 1). Миграции при старте, репозитории.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/zaaalex/servys/backend/crypto"
	"github.com/zaaalex/servys/backend/domain"

	_ "modernc.org/sqlite"
)

// ErrNotFound — сущность не найдена (или принадлежит другому пользователю).
var ErrNotFound = errors.New("not found")

// ErrOdometerDecrease — попытка уменьшить пробег (ADR-001 §S07).
var ErrOdometerDecrease = errors.New("odometer decrease not allowed")

type Store struct {
	db     *sql.DB
	cipher *crypto.Cipher // для шифрования вебхуков СТО (b2b); nil => b2b-методы недоступны
}

// SetCipher включает шифрование секретов b2b (вебхуков). Вызывается из main при наличии APP_SECRET_KEY.
func (s *Store) SetCipher(c *crypto.Cipher) { s.cipher = c }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
  id          TEXT PRIMARY KEY,
  client_key  TEXT UNIQUE NOT NULL,
  tenant_type TEXT NOT NULL DEFAULT 'b2c',
  created_at  DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS vehicles (
  id                    TEXT PRIMARY KEY,
  user_id               TEXT NOT NULL,
  vin                   TEXT,
  make                  TEXT NOT NULL,
  model                 TEXT NOT NULL,
  year                  INTEGER,
  engine_cc             INTEGER,
  power_hp              INTEGER,
  color                 TEXT,
  body_type             TEXT,
  identification_source TEXT NOT NULL,
  current_odometer      INTEGER NOT NULL,
  odometer_updated_at   DATETIME NOT NULL,
  created_at            DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS odometer_readings (
  id         TEXT PRIMARY KEY,
  vehicle_id TEXT NOT NULL,
  value      INTEGER NOT NULL,
  recorded_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS service_events (
  id           TEXT PRIMARY KEY,
  vehicle_id   TEXT NOT NULL,
  rule_code    TEXT NOT NULL,
  odometer     INTEGER NOT NULL,
  performed_at DATETIME NOT NULL,
  created_at   DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS service_centers (
  id             TEXT PRIMARY KEY,
  name           TEXT NOT NULL,
  webhook_enc    TEXT NOT NULL,
  responsible_id INTEGER NOT NULL DEFAULT 1,
  created_at     DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS pushed_actions (
  id         TEXT PRIMARY KEY,
  tenant_id  TEXT NOT NULL,
  dedupe_key TEXT NOT NULL,
  remote_id  TEXT,
  created_at DATETIME NOT NULL,
  UNIQUE(tenant_id, dedupe_key)
);
CREATE TABLE IF NOT EXISTS accounts (
  id         TEXT PRIMARY KEY,
  created_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS identities (
  id          TEXT PRIMARY KEY,
  account_id  TEXT NOT NULL,
  provider    TEXT NOT NULL,
  external_id TEXT NOT NULL,
  secret      TEXT NOT NULL DEFAULT '',
  UNIQUE(provider, external_id)
);
CREATE TABLE IF NOT EXISTS memberships (
  id         TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  ctx_type   TEXT NOT NULL,
  tenant_id  TEXT NOT NULL DEFAULT '',
  role       TEXT NOT NULL DEFAULT '',
  UNIQUE(account_id, ctx_type, tenant_id)
);
CREATE TABLE IF NOT EXISTS refresh_tokens (
  token_hash TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  expires_at DATETIME NOT NULL,
  revoked    INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_identities_account ON identities(account_id);
CREATE INDEX IF NOT EXISTS idx_memberships_account ON memberships(account_id);
CREATE INDEX IF NOT EXISTS idx_vehicles_user ON vehicles(user_id);
CREATE INDEX IF NOT EXISTS idx_readings_vehicle ON odometer_readings(vehicle_id);
CREATE INDEX IF NOT EXISTS idx_service_vehicle ON service_events(vehicle_id);
`)
	return err
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// EnsureUser возвращает пользователя по client-token, создавая при первом обращении.
func (s *Store) EnsureUser(ctx context.Context, clientKey string) (domain.User, error) {
	var u domain.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, client_key, tenant_type, created_at FROM users WHERE client_key = ?`, clientKey).
		Scan(&u.ID, &u.ClientKey, &u.TenantType, &u.CreatedAt)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, err
	}
	u = domain.User{ID: newID(), ClientKey: clientKey, TenantType: domain.TenantB2C, CreatedAt: time.Now().UTC()}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO users(id, client_key, tenant_type, created_at) VALUES(?,?,?,?)`,
		u.ID, u.ClientKey, u.TenantType, u.CreatedAt); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// AddVehicle сохраняет авто и первое показание пробега.
func (s *Store) AddVehicle(ctx context.Context, v domain.Vehicle) (domain.Vehicle, error) {
	v.ID = newID()
	now := time.Now().UTC()
	v.OdometerUpdatedAt = now
	if v.IdentificationSource == "" {
		v.IdentificationSource = "manual"
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO vehicles(id,user_id,vin,make,model,year,engine_cc,power_hp,color,body_type,identification_source,current_odometer,odometer_updated_at,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		v.ID, v.UserID, v.VIN, v.Make, v.Model, v.Year, v.EngineCC, v.PowerHP, v.Color, v.BodyType,
		v.IdentificationSource, v.CurrentOdometer, v.OdometerUpdatedAt, now); err != nil {
		return domain.Vehicle{}, err
	}
	if err := s.addReading(ctx, v.ID, v.CurrentOdometer, now); err != nil {
		return domain.Vehicle{}, err
	}
	return v, nil
}

func (s *Store) addReading(ctx context.Context, vehicleID string, value int, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO odometer_readings(id, vehicle_id, value, recorded_at) VALUES(?,?,?,?)`,
		newID(), vehicleID, value, at)
	return err
}

func (s *Store) ListVehicles(ctx context.Context, userID string) ([]domain.Vehicle, error) {
	rows, err := s.db.QueryContext(ctx, vehicleCols+` WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Vehicle
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetVehicle возвращает авто, только если оно принадлежит userID (иначе ErrNotFound).
func (s *Store) GetVehicle(ctx context.Context, userID, id string) (domain.Vehicle, error) {
	row := s.db.QueryRowContext(ctx, vehicleCols+` WHERE id = ? AND user_id = ?`, id, userID)
	v, err := scanVehicle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Vehicle{}, ErrNotFound
	}
	return v, err
}

// UpdateOdometer обновляет пробег (запрещая уменьшение) и пишет замер в историю.
func (s *Store) UpdateOdometer(ctx context.Context, userID, id string, value int) (domain.Vehicle, error) {
	v, err := s.GetVehicle(ctx, userID, id)
	if err != nil {
		return domain.Vehicle{}, err
	}
	if value < v.CurrentOdometer {
		return domain.Vehicle{}, ErrOdometerDecrease
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx,
		`UPDATE vehicles SET current_odometer = ?, odometer_updated_at = ? WHERE id = ?`, value, now, id); err != nil {
		return domain.Vehicle{}, err
	}
	if err := s.addReading(ctx, id, value, now); err != nil {
		return domain.Vehicle{}, err
	}
	v.CurrentOdometer = value
	v.OdometerUpdatedAt = now
	return v, nil
}

// AddServiceEvent записывает подтверждённое ТО (проверяя, что авто принадлежит userID).
func (s *Store) AddServiceEvent(ctx context.Context, userID, vehicleID string, ev domain.ServiceEvent) (domain.ServiceEvent, error) {
	if _, err := s.GetVehicle(ctx, userID, vehicleID); err != nil {
		return domain.ServiceEvent{}, err
	}
	ev.ID = newID()
	ev.VehicleID = vehicleID
	if ev.PerformedAt.IsZero() {
		ev.PerformedAt = time.Now().UTC()
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO service_events(id, vehicle_id, rule_code, odometer, performed_at, created_at) VALUES(?,?,?,?,?,?)`,
		ev.ID, ev.VehicleID, ev.RuleCode, ev.Odometer, ev.PerformedAt, time.Now().UTC()); err != nil {
		return domain.ServiceEvent{}, err
	}
	return ev, nil
}

// ListServiceEvents возвращает историю ТО авто (проверяя владельца), свежее — раньше.
func (s *Store) ListServiceEvents(ctx context.Context, userID, vehicleID string) ([]domain.ServiceEvent, error) {
	if _, err := s.GetVehicle(ctx, userID, vehicleID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, vehicle_id, rule_code, odometer, performed_at FROM service_events WHERE vehicle_id = ? ORDER BY performed_at DESC`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ServiceEvent
	for rows.Next() {
		var e domain.ServiceEvent
		if err := rows.Scan(&e.ID, &e.VehicleID, &e.RuleCode, &e.Odometer, &e.PerformedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

const vehicleCols = `SELECT id,user_id,vin,make,model,year,engine_cc,power_hp,color,body_type,identification_source,current_odometer,odometer_updated_at FROM vehicles`

type scanner interface{ Scan(dest ...any) error }

func scanVehicle(sc scanner) (domain.Vehicle, error) {
	var v domain.Vehicle
	var vin, color, body sql.NullString
	var year, cc, hp sql.NullInt64
	if err := sc.Scan(&v.ID, &v.UserID, &vin, &v.Make, &v.Model, &year, &cc, &hp,
		&color, &body, &v.IdentificationSource, &v.CurrentOdometer, &v.OdometerUpdatedAt); err != nil {
		return domain.Vehicle{}, err
	}
	v.VIN, v.Color, v.BodyType = vin.String, color.String, body.String
	v.Year, v.EngineCC, v.PowerHP = int(year.Int64), int(cc.Int64), int(hp.Int64)
	return v, nil
}
