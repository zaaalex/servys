package store

import (
	"context"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

// PulseDay — один день на пульте: сколько людей и машин появилось.
type PulseDay struct {
	Date  string
	Users int
	Cars  int
}

// AdminPulse считает людей из Telegram, машины и последние семь дней.
func (s *Store) AdminPulse(ctx context.Context) (users, cars int, days []PulseDay, err error) {
	if err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM identities WHERE provider = ?`, domain.ProviderTelegram).Scan(&users); err != nil {
		return 0, 0, nil, err
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vehicles`).Scan(&cars); err != nil {
		return 0, 0, nil, err
	}
	userTimes, err := s.createdAt(ctx, `
		SELECT a.created_at FROM identities i
		JOIN accounts a ON a.id = i.account_id
		WHERE i.provider = ?`, domain.ProviderTelegram)
	if err != nil {
		return 0, 0, nil, err
	}
	carTimes, err := s.createdAt(ctx, `SELECT created_at FROM vehicles`)
	if err != nil {
		return 0, 0, nil, err
	}
	today := time.Now().UTC()
	days = make([]PulseDay, 7)
	for i := 0; i < 7; i++ {
		day := today.AddDate(0, 0, i-6)
		key := day.Format("2006-01-02")
		days[i] = PulseDay{Date: key, Users: countDay(userTimes, key), Cars: countDay(carTimes, key)}
	}
	return users, cars, days, nil
}

func (s *Store) createdAt(ctx context.Context, query string, args ...any) ([]time.Time, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if t, ok := parseSQLTime(raw); ok {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

func countDay(times []time.Time, key string) int {
	n := 0
	for _, t := range times {
		if t.UTC().Format("2006-01-02") == key {
			n++
		}
	}
	return n
}

func parseSQLTime(raw string) (time.Time, bool) {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
