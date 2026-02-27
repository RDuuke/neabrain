package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"neabrain/internal/domain"
	ports "neabrain/internal/ports/outbound"
)

type DuplicateRepository struct {
	db *sql.DB
}

var _ ports.DuplicateRepository = (*DuplicateRepository)(nil)

func NewDuplicateRepository(db *sql.DB) *DuplicateRepository {
	return &DuplicateRepository{db: db}
}

func (r *DuplicateRepository) Create(ctx context.Context, duplicate domain.Duplicate) (domain.Duplicate, error) {
	if strings.TrimSpace(duplicate.ID) == "" {
		return domain.Duplicate{}, domain.NewInvalidInput("duplicate id is required")
	}

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO duplicates (id, original_observation_id, duplicate_observation_id, reason, created_at)
		VALUES (?, ?, ?, ?, ?);`,
		duplicate.ID,
		duplicate.OriginalObservationID,
		duplicate.DuplicateObservationID,
		duplicate.Reason,
		formatTime(duplicate.CreatedAt),
	)
	if err != nil {
		if isConstraintError(err) {
			return domain.Duplicate{}, domain.NewConflict("duplicate record already exists")
		}
		return domain.Duplicate{}, err
	}

	return duplicate, nil
}

func (r *DuplicateRepository) ListByObservationID(ctx context.Context, observationID string) ([]domain.Duplicate, error) {
	if strings.TrimSpace(observationID) == "" {
		return nil, domain.NewInvalidInput("observation id is required")
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, original_observation_id, duplicate_observation_id, reason, created_at
		FROM duplicates
		WHERE original_observation_id = ? OR duplicate_observation_id = ?
		ORDER BY created_at DESC;`,
		observationID,
		observationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var duplicates []domain.Duplicate
	for rows.Next() {
		duplicate, err := scanDuplicate(rows)
		if err != nil {
			return nil, err
		}
		duplicates = append(duplicates, duplicate)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return duplicates, nil
}

func scanDuplicate(scanner interface {
	Scan(dest ...any) error
}) (domain.Duplicate, error) {
	var (
		duplicate domain.Duplicate
		createdAt string
	)

	if err := scanner.Scan(
		&duplicate.ID,
		&duplicate.OriginalObservationID,
		&duplicate.DuplicateObservationID,
		&duplicate.Reason,
		&createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Duplicate{}, domain.NewNotFound("duplicate record not found")
		}
		return domain.Duplicate{}, err
	}

	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return domain.Duplicate{}, err
	}
	duplicate.CreatedAt = parsedCreatedAt

	return duplicate, nil
}
