package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"neabrain/internal/domain"
	ports "neabrain/internal/ports/outbound"
)

type SessionRepository struct {
	db *sql.DB
}

var _ ports.SessionRepository = (*SessionRepository)(nil)

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, session domain.Session) (domain.Session, error) {
	if strings.TrimSpace(session.ID) == "" {
		return domain.Session{}, domain.NewInvalidInput("session id is required")
	}

	encodedOperations, err := marshalStringList(session.RecentOperations)
	if err != nil {
		return domain.Session{}, err
	}

	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO sessions (id, created_at, last_seen_at, disclosure_level, recent_operations)
		VALUES (?, ?, ?, ?, ?);`,
		session.ID,
		formatTime(session.CreatedAt),
		formatTime(session.LastSeenAt),
		session.DisclosureLevel,
		encodedOperations,
	)
	if err != nil {
		if isConstraintError(err) {
			return domain.Session{}, domain.NewConflict("session already exists")
		}
		return domain.Session{}, err
	}

	return session, nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id string) (domain.Session, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, created_at, last_seen_at, disclosure_level, recent_operations
		FROM sessions WHERE id = ?;`,
		id,
	)

	return scanSession(row)
}

func (r *SessionRepository) Update(ctx context.Context, session domain.Session) (domain.Session, error) {
	encodedOperations, err := marshalStringList(session.RecentOperations)
	if err != nil {
		return domain.Session{}, err
	}

	result, err := r.db.ExecContext(
		ctx,
		`UPDATE sessions
		SET last_seen_at = ?, disclosure_level = ?, recent_operations = ?
		WHERE id = ?;`,
		formatTime(session.LastSeenAt),
		session.DisclosureLevel,
		encodedOperations,
		session.ID,
	)
	if err != nil {
		return domain.Session{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Session{}, err
	}
	if affected == 0 {
		return domain.Session{}, domain.NewNotFound("session not found")
	}

	return session, nil
}

func scanSession(scanner interface {
	Scan(dest ...any) error
}) (domain.Session, error) {
	var (
		session          domain.Session
		createdAt        string
		lastSeenAt       string
		recentOperations sql.NullString
	)

	if err := scanner.Scan(
		&session.ID,
		&createdAt,
		&lastSeenAt,
		&session.DisclosureLevel,
		&recentOperations,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Session{}, domain.NewNotFound("session not found")
		}
		return domain.Session{}, err
	}

	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return domain.Session{}, err
	}
	parsedLastSeenAt, err := parseTime(lastSeenAt)
	if err != nil {
		return domain.Session{}, err
	}
	parsedOperations, err := unmarshalStringList(recentOperations)
	if err != nil {
		return domain.Session{}, err
	}

	session.CreatedAt = parsedCreatedAt
	session.LastSeenAt = parsedLastSeenAt
	session.RecentOperations = parsedOperations

	return session, nil
}
