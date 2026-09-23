package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// InsertAudit appends an immutable audit_logs row.
func (s *Store) InsertAudit(ctx context.Context, actorType string, actorID *uuid.UUID, action, subjectType string, subjectID *uuid.UUID, detail any) error {
	var raw []byte
	if detail != nil {
		b, err := json.Marshal(detail)
		if err != nil {
			return err
		}
		raw = b
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_type, actor_id, action, subject_type, subject_id, detail)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, actorType, actorID, action, nullIfEmpty(subjectType), subjectID, raw)
	if err != nil {
		return mapQueryErr("store.insertAudit", err)
	}
	return nil
}

// CountAudit is a test helper.
func (s *Store) CountAudit(ctx context.Context, action string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = $1`, action).Scan(&n)
	if err != nil {
		return 0, mapQueryErr("store.countAudit", err)
	}
	return n, nil
}
