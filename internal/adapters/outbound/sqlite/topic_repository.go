package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"neabrain/internal/domain"
	ports "neabrain/internal/ports/outbound"
)

type TopicRepository struct {
	db *sql.DB
}

var _ ports.TopicRepository = (*TopicRepository)(nil)

func NewTopicRepository(db *sql.DB) *TopicRepository {
	return &TopicRepository{db: db}
}

func (r *TopicRepository) UpsertByTopicKey(ctx context.Context, topic domain.Topic) (domain.Topic, error) {
	if strings.TrimSpace(topic.TopicKey) == "" {
		return domain.Topic{}, domain.NewInvalidInput("topic key is required")
	}

	existing, err := r.GetByTopicKey(ctx, topic.TopicKey)
	if err == nil {
		topic.ID = existing.ID
		topic.CreatedAt = existing.CreatedAt
		return r.update(ctx, topic)
	}

	var domainErr domain.DomainError
	if errors.As(err, &domainErr) && domainErr.Code == domain.ErrorNotFound {
		return r.insert(ctx, topic)
	}

	return domain.Topic{}, err
}

func (r *TopicRepository) GetByTopicKey(ctx context.Context, topicKey string) (domain.Topic, error) {
	row := r.db.QueryRowContext(
		ctx,
		`SELECT id, topic_key, name, description, metadata, created_at, updated_at
		FROM topics WHERE topic_key = ?;`,
		topicKey,
	)

	return scanTopic(row)
}

func (r *TopicRepository) List(ctx context.Context) ([]domain.TopicSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			t.topic_key,
			t.name,
			t.description,
			COUNT(o.id) AS cnt
		FROM topics t
		LEFT JOIN observations o
			ON o.topic_key = t.topic_key
			AND o.deleted_at IS NULL
		GROUP BY t.topic_key, t.name, t.description
		ORDER BY cnt DESC, t.topic_key ASC;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []domain.TopicSummary
	for rows.Next() {
		var summary domain.TopicSummary
		if err := rows.Scan(&summary.TopicKey, &summary.Name, &summary.Description, &summary.Count); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	return summaries, rows.Err()
}

func (r *TopicRepository) insert(ctx context.Context, topic domain.Topic) (domain.Topic, error) {
	if strings.TrimSpace(topic.ID) == "" {
		return domain.Topic{}, domain.NewInvalidInput("topic id is required")
	}

	encodedMetadata, err := marshalMetadata(topic.Metadata)
	if err != nil {
		return domain.Topic{}, err
	}

	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO topics (id, topic_key, name, description, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?);`,
		topic.ID,
		topic.TopicKey,
		topic.Name,
		topic.Description,
		encodedMetadata,
		formatTime(topic.CreatedAt),
		formatTime(topic.UpdatedAt),
	)
	if err != nil {
		if isConstraintError(err) {
			return domain.Topic{}, domain.NewConflict("topic already exists")
		}
		return domain.Topic{}, err
	}

	return topic, nil
}

func (r *TopicRepository) update(ctx context.Context, topic domain.Topic) (domain.Topic, error) {
	encodedMetadata, err := marshalMetadata(topic.Metadata)
	if err != nil {
		return domain.Topic{}, err
	}

	result, err := r.db.ExecContext(
		ctx,
		`UPDATE topics
		SET name = ?, description = ?, metadata = ?, updated_at = ?
		WHERE topic_key = ?;`,
		topic.Name,
		topic.Description,
		encodedMetadata,
		formatTime(topic.UpdatedAt),
		topic.TopicKey,
	)
	if err != nil {
		return domain.Topic{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Topic{}, err
	}
	if affected == 0 {
		return domain.Topic{}, domain.NewNotFound("topic not found")
	}

	return topic, nil
}

func scanTopic(scanner interface {
	Scan(dest ...any) error
}) (domain.Topic, error) {
	var (
		topic     domain.Topic
		metadata  sql.NullString
		createdAt string
		updatedAt string
	)

	if err := scanner.Scan(
		&topic.ID,
		&topic.TopicKey,
		&topic.Name,
		&topic.Description,
		&metadata,
		&createdAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Topic{}, domain.NewNotFound("topic not found")
		}
		return domain.Topic{}, err
	}

	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return domain.Topic{}, err
	}
	parsedUpdatedAt, err := parseTime(updatedAt)
	if err != nil {
		return domain.Topic{}, err
	}
	parsedMetadata, err := unmarshalMetadata(metadata)
	if err != nil {
		return domain.Topic{}, err
	}

	topic.CreatedAt = parsedCreatedAt
	topic.UpdatedAt = parsedUpdatedAt
	topic.Metadata = parsedMetadata

	return topic, nil
}
