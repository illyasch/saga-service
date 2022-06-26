package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Storage struct {
	db *sqlx.DB
}

func NewStorage(db *sqlx.DB) Storage {
	return Storage{
		db: db,
	}
}

func (s Storage) InsertSaga(ctx context.Context, sagaID uuid.UUID, service, status string) error {
	// To keep the operation idempotent we do nothing if the saga has been already started.
	const query = `INSERT INTO sagas(id, status, service, date_created) VALUES ($1, $2, $3, NOW()) 
                	ON CONFLICT(id) DO NOTHING`

	if _, err := s.db.ExecContext(ctx, query, sagaID, status, service); err != nil {
		return fmt.Errorf("query %s: %w", query, err)
	}

	return nil
}

func (s Storage) UpdateStatus(ctx context.Context, sagaID uuid.UUID, status string) error {
	const query = `UPDATE sagas SET status = $1 WHERE id = $2`
	res, err := s.db.ExecContext(ctx, query, status, sagaID)
	if err != nil {
		return fmt.Errorf("query %s: %w", query, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("affected rows: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s Storage) UpdateService(ctx context.Context, sagaID uuid.UUID, prev, next string) error {
	// Updates the service in the saga to next only if it is set to the previous service in DB.
	const query = `UPDATE sagas SET service = $1 WHERE id = $2 AND service = $3`
	if _, err := s.db.ExecContext(ctx, query, next, sagaID, prev); err != nil {
		return fmt.Errorf("query %s: %w", query, err)
	}

	return nil
}
