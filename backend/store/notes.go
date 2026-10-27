package store

import (
	"context"
	"time"
)

// Note — сохранённый текст или файл истории обслуживания.
type Note struct {
	ID        string
	VehicleID string
	Body      string
	Filename  string
	CreatedAt time.Time
}

func (s *Store) AddNote(ctx context.Context, accountID, vehicleID, body, filename string) (Note, error) {
	if _, err := s.GetVehicle(ctx, accountID, vehicleID); err != nil {
		return Note{}, err
	}
	n := Note{
		ID:        newID(),
		VehicleID: vehicleID,
		Body:      body,
		Filename:  filename,
		CreatedAt: time.Now().UTC(),
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO maintenance_notes(id, vehicle_id, body, filename, created_at) VALUES(?,?,?,?,?)`,
		n.ID, n.VehicleID, n.Body, n.Filename, n.CreatedAt)
	if err != nil {
		return Note{}, err
	}
	return n, nil
}

func (s *Store) ListNotes(ctx context.Context, accountID, vehicleID string) ([]Note, error) {
	if _, err := s.GetVehicle(ctx, accountID, vehicleID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, vehicle_id, body, filename, created_at FROM maintenance_notes WHERE vehicle_id = ? ORDER BY created_at DESC`,
		vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.VehicleID, &n.Body, &n.Filename, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
