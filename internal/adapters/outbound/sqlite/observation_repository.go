package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"neabrain/internal/domain"
	ports "neabrain/internal/ports/outbound"
)

type ObservationRepository struct {
	db *sql.DB
}

var _ ports.ObservationRepository = (*ObservationRepository)(nil)

func NewObservationRepository(db *sql.DB) *ObservationRepository {
	return &ObservationRepository{db: db}
}

func (r *ObservationRepository) Create(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	if strings.TrimSpace(observation.ID) == "" {
		return domain.Observation{}, domain.NewInvalidInput("observation id is required")
	}

	encodedTags, err := marshalTags(observation.Tags)
	if err != nil {
		return domain.Observation{}, err
	}
	encodedMetadata, err := marshalMetadata(observation.Metadata)
	if err != nil {
		return domain.Observation{}, err
	}

	var deletedAt any
	if observation.DeletedAt != nil {
		deletedAt = formatTime(*observation.DeletedAt)
	}

	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO observations (id, content, created_at, updated_at, deleted_at, project, topic_key, tags, source, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		observation.ID,
		observation.Content,
		formatTime(observation.CreatedAt),
		formatTime(observation.UpdatedAt),
		deletedAt,
		observation.Project,
		observation.TopicKey,
		encodedTags,
		observation.Source,
		encodedMetadata,
	)
	if err != nil {
		if isConstraintError(err) {
			return domain.Observation{}, domain.NewConflict("observation already exists")
		}
		return domain.Observation{}, err
	}

	return observation, nil
}

func (r *ObservationRepository) GetByID(ctx context.Context, id string, includeDeleted bool) (domain.Observation, error) {
	query := `SELECT id, content, created_at, updated_at, deleted_at, project, topic_key, tags, source, metadata
		FROM observations WHERE id = ?`
	if !includeDeleted {
		query += " AND deleted_at IS NULL"
	}

	row := r.db.QueryRowContext(ctx, query, id)
	return scanObservation(row)
}

func (r *ObservationRepository) Update(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	encodedTags, err := marshalTags(observation.Tags)
	if err != nil {
		return domain.Observation{}, err
	}
	encodedMetadata, err := marshalMetadata(observation.Metadata)
	if err != nil {
		return domain.Observation{}, err
	}

	var deletedAt any
	if observation.DeletedAt != nil {
		deletedAt = formatTime(*observation.DeletedAt)
	}

	result, err := r.db.ExecContext(
		ctx,
		`UPDATE observations
		SET content = ?, updated_at = ?, deleted_at = ?, project = ?, topic_key = ?, tags = ?, source = ?, metadata = ?
		WHERE id = ?;`,
		observation.Content,
		formatTime(observation.UpdatedAt),
		deletedAt,
		observation.Project,
		observation.TopicKey,
		encodedTags,
		observation.Source,
		encodedMetadata,
		observation.ID,
	)
	if err != nil {
		return domain.Observation{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Observation{}, err
	}
	if affected == 0 {
		return domain.Observation{}, domain.NewNotFound("observation not found")
	}

	return observation, nil
}

func (r *ObservationRepository) List(ctx context.Context, filter ports.ObservationFilter) ([]domain.Observation, error) {
	query := `SELECT id, content, created_at, updated_at, deleted_at, project, topic_key, tags, source, metadata
		FROM observations WHERE 1=1`
	args := make([]any, 0, 3)

	if filter.Project != "" {
		query += " AND project = ?"
		args = append(args, filter.Project)
	}
	if filter.TopicKey != "" {
		query += " AND topic_key = ?"
		args = append(args, filter.TopicKey)
	}
	if !filter.IncludeDeleted {
		query += " AND deleted_at IS NULL"
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var observations []domain.Observation
	for rows.Next() {
		observation, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}

		if len(filter.Tags) > 0 && !containsAllTags(observation.Tags, filter.Tags) {
			continue
		}

		observations = append(observations, observation)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return observations, nil
}

func (r *ObservationRepository) FindAround(ctx context.Context, id string, before, after int, includeDeleted bool) (domain.TimelineResult, error) {
	target, err := r.GetByID(ctx, id, includeDeleted)
	if err != nil {
		return domain.TimelineResult{}, err
	}

	result := domain.TimelineResult{Target: target}

	if before > 0 {
		query := `SELECT id, content, created_at, updated_at, deleted_at, project, topic_key, tags, source, metadata
			FROM observations
			WHERE ((created_at < ?) OR (created_at = ? AND id < ?))`
		args := []any{formatTime(target.CreatedAt), formatTime(target.CreatedAt), target.ID}
		if !includeDeleted {
			query += " AND deleted_at IS NULL"
		}
		query += " ORDER BY created_at DESC, id DESC LIMIT ?"
		args = append(args, before)

		observations, err := r.queryObservations(ctx, query, args...)
		if err != nil {
			return domain.TimelineResult{}, err
		}
		reverseObservations(observations)
		result.Before = observations
	}

	if after > 0 {
		query := `SELECT id, content, created_at, updated_at, deleted_at, project, topic_key, tags, source, metadata
			FROM observations
			WHERE ((created_at > ?) OR (created_at = ? AND id > ?))`
		args := []any{formatTime(target.CreatedAt), formatTime(target.CreatedAt), target.ID}
		if !includeDeleted {
			query += " AND deleted_at IS NULL"
		}
		query += " ORDER BY created_at ASC, id ASC LIMIT ?"
		args = append(args, after)

		observations, err := r.queryObservations(ctx, query, args...)
		if err != nil {
			return domain.TimelineResult{}, err
		}
		result.After = observations
	}

	return result, nil
}

func (r *ObservationRepository) SoftDelete(ctx context.Context, id string, deletedAt time.Time) (domain.Observation, error) {
	result, err := r.db.ExecContext(
		ctx,
		"UPDATE observations SET deleted_at = ?, updated_at = ? WHERE id = ?;",
		formatTime(deletedAt),
		formatTime(deletedAt),
		id,
	)
	if err != nil {
		return domain.Observation{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Observation{}, err
	}
	if affected == 0 {
		return domain.Observation{}, domain.NewNotFound("observation not found")
	}

	return r.GetByID(ctx, id, true)
}

func (r *ObservationRepository) ListProjects(ctx context.Context) ([]domain.ProjectSummary, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT project, COUNT(*) AS cnt FROM observations WHERE deleted_at IS NULL AND project != '' GROUP BY project ORDER BY cnt DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []domain.ProjectSummary
	for rows.Next() {
		var s domain.ProjectSummary
		if err := rows.Scan(&s.Name, &s.Count); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func (r *ObservationRepository) RenameProject(ctx context.Context, oldName, newName string) (int, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE observations SET project = ?, updated_at = updated_at WHERE project = ?`, newName, oldName)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func (r *ObservationRepository) GetStats(ctx context.Context) (domain.ObservationStats, error) {
	var stats domain.ObservationStats
	row := r.db.QueryRowContext(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE deleted_at IS NULL),
			COUNT(*) FILTER (WHERE deleted_at IS NOT NULL),
			COUNT(DISTINCT project) FILTER (WHERE deleted_at IS NULL AND project != '')
		FROM observations`)
	if err := row.Scan(&stats.Active, &stats.Deleted, &stats.Projects); err != nil {
		return domain.ObservationStats{}, err
	}
	return stats, nil
}

func (r *ObservationRepository) FindByContent(ctx context.Context, content string, project string, includeDeleted bool) ([]domain.Observation, error) {
	query := `SELECT id, content, created_at, updated_at, deleted_at, project, topic_key, tags, source, metadata
		FROM observations WHERE content = ? AND project = ?`
	args := []any{content, project}
	if !includeDeleted {
		query += " AND deleted_at IS NULL"
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var observations []domain.Observation
	for rows.Next() {
		observation, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return observations, nil
}

func (r *ObservationRepository) queryObservations(ctx context.Context, query string, args ...any) ([]domain.Observation, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var observations []domain.Observation
	for rows.Next() {
		observation, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return observations, nil
}

func reverseObservations(observations []domain.Observation) {
	for left, right := 0, len(observations)-1; left < right; left, right = left+1, right-1 {
		observations[left], observations[right] = observations[right], observations[left]
	}
}

func scanObservation(scanner interface {
	Scan(dest ...any) error
}) (domain.Observation, error) {
	var (
		observation domain.Observation
		createdAt   string
		updatedAt   string
		deletedAt   sql.NullString
		tags        sql.NullString
		metadata    sql.NullString
	)

	if err := scanner.Scan(
		&observation.ID,
		&observation.Content,
		&createdAt,
		&updatedAt,
		&deletedAt,
		&observation.Project,
		&observation.TopicKey,
		&tags,
		&observation.Source,
		&metadata,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Observation{}, domain.NewNotFound("observation not found")
		}
		return domain.Observation{}, err
	}

	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return domain.Observation{}, err
	}
	parsedUpdatedAt, err := parseTime(updatedAt)
	if err != nil {
		return domain.Observation{}, err
	}
	parsedDeletedAt, err := parseNullableTime(deletedAt)
	if err != nil {
		return domain.Observation{}, err
	}
	parsedTags, err := unmarshalTags(tags)
	if err != nil {
		return domain.Observation{}, err
	}
	parsedMetadata, err := unmarshalMetadata(metadata)
	if err != nil {
		return domain.Observation{}, err
	}

	observation.CreatedAt = parsedCreatedAt
	observation.UpdatedAt = parsedUpdatedAt
	observation.DeletedAt = parsedDeletedAt
	observation.Tags = parsedTags
	observation.Metadata = parsedMetadata

	return observation, nil
}

func containsAllTags(haystack []string, needles []string) bool {
	if len(needles) == 0 {
		return true
	}

	set := make(map[string]struct{}, len(haystack))
	for _, tag := range haystack {
		set[tag] = struct{}{}
	}

	for _, needle := range needles {
		if _, ok := set[needle]; !ok {
			return false
		}
	}

	return true
}

func isConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "constraint") || strings.Contains(message, "unique")
}
