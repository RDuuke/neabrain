package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"unicode"

	"neabrain/internal/domain"
	ports "neabrain/internal/ports/outbound"
)

type SearchIndex struct {
	db *sql.DB
}

var _ ports.SearchIndex = (*SearchIndex)(nil)

func NewSearchIndex(db *sql.DB) *SearchIndex {
	return &SearchIndex{db: db}
}

func (s *SearchIndex) Index(ctx context.Context, observation domain.Observation) error {
	if strings.TrimSpace(observation.ID) == "" {
		return domain.NewInvalidInput("observation id is required")
	}
	if strings.TrimSpace(observation.Content) == "" {
		return domain.NewInvalidInput("observation content is required")
	}

	if _, err := s.db.ExecContext(ctx, "DELETE FROM observations_fts WHERE observation_id = ?;", observation.ID); err != nil {
		if shouldRebuildFTS(err) {
			if rebuildErr := s.Rebuild(ctx); rebuildErr == nil {
				_, retryErr := s.db.ExecContext(ctx, "DELETE FROM observations_fts WHERE observation_id = ?;", observation.ID)
				return retryErr
			}
		}
		return err
	}

	_, err := s.db.ExecContext(
		ctx,
		"INSERT INTO observations_fts (observation_id, content) VALUES (?, ?);",
		observation.ID,
		observation.Content,
	)
	if err != nil && shouldRebuildFTS(err) {
		if rebuildErr := s.Rebuild(ctx); rebuildErr == nil {
			_, retryErr := s.db.ExecContext(
				ctx,
				"INSERT INTO observations_fts (observation_id, content) VALUES (?, ?);",
				observation.ID,
				observation.Content,
			)
			return retryErr
		}
	}
	return err
}

func (s *SearchIndex) Remove(ctx context.Context, observationID string) error {
	if strings.TrimSpace(observationID) == "" {
		return domain.NewInvalidInput("observation id is required")
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM observations_fts WHERE observation_id = ?;", observationID)
	if err != nil && shouldRebuildFTS(err) {
		if rebuildErr := s.Rebuild(ctx); rebuildErr == nil {
			_, retryErr := s.db.ExecContext(ctx, "DELETE FROM observations_fts WHERE observation_id = ?;", observationID)
			return retryErr
		}
	}
	return err
}

func (s *SearchIndex) Search(ctx context.Context, query string, filter ports.SearchFilter) ([]ports.SearchResult, error) {
	normalized := normalizeFTSQuery(query)
	if normalized == "" {
		return nil, domain.NewInvalidInput("search query is required")
	}

	sqlQuery := `SELECT f.observation_id, bm25(observations_fts) AS score, o.tags
		FROM observations_fts f
		JOIN observations o ON o.id = f.observation_id
		WHERE observations_fts MATCH ?`
	args := []any{normalized}

	if filter.Project != "" {
		sqlQuery += " AND o.project = ?"
		args = append(args, filter.Project)
	}
	if filter.TopicKey != "" {
		sqlQuery += " AND o.topic_key = ?"
		args = append(args, filter.TopicKey)
	}
	if !filter.IncludeDeleted {
		sqlQuery += " AND o.deleted_at IS NULL"
	}
	sqlQuery += " ORDER BY score ASC"

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		if shouldRebuildFTS(err) {
			if rebuildErr := s.Rebuild(ctx); rebuildErr == nil {
				rows, err = s.db.QueryContext(ctx, sqlQuery, args...)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var results []ports.SearchResult
	for rows.Next() {
		var (
			observationID string
			score         float64
			tags          sql.NullString
		)
		if err := rows.Scan(&observationID, &score, &tags); err != nil {
			return nil, err
		}

		if len(filter.Tags) > 0 {
			parsedTags, err := unmarshalTags(tags)
			if err != nil {
				return nil, err
			}
			if !containsAllTags(parsedTags, filter.Tags) {
				continue
			}
		}

		results = append(results, ports.SearchResult{ObservationID: observationID, Score: score})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// Rebuild recreates and repopulates the FTS index from observations.
func (s *SearchIndex) Rebuild(ctx context.Context) error {
	if s == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(content, observation_id UNINDEXED);"); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM observations_fts;"); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO observations_fts (observation_id, content) SELECT id, content FROM observations;"); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func normalizeFTSQuery(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ""
	}
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return r
		}
		return ' '
	}, trimmed)
	parts := strings.Fields(cleaned)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " AND ")
}

func shouldRebuildFTS(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table: observations_fts") || strings.Contains(msg, "database disk image is malformed")
}
