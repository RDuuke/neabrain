package domain

import (
	"context"
	"time"
)

// ObservationRepository defines persistence operations for observations.
type ObservationRepository interface {
	Create(ctx context.Context, observation Observation) (Observation, error)
	GetByID(ctx context.Context, id string, includeDeleted bool) (Observation, error)
	Update(ctx context.Context, observation Observation) (Observation, error)
	List(ctx context.Context, filter ObservationListFilter) ([]Observation, error)
	SoftDelete(ctx context.Context, id string, deletedAt time.Time) (Observation, error)
	FindByContent(ctx context.Context, content string, project string, includeDeleted bool) ([]Observation, error)
	ListProjects(ctx context.Context) ([]ProjectSummary, error)
	RenameProject(ctx context.Context, oldName, newName string) (int, error)
	GetStats(ctx context.Context) (ObservationStats, error)
}

// TopicRepository defines persistence operations for topics.
type TopicRepository interface {
	UpsertByTopicKey(ctx context.Context, topic Topic) (Topic, error)
	GetByTopicKey(ctx context.Context, topicKey string) (Topic, error)
}

// SessionRepository defines persistence operations for sessions.
type SessionRepository interface {
	Create(ctx context.Context, session Session) (Session, error)
	GetByID(ctx context.Context, id string) (Session, error)
	Update(ctx context.Context, session Session) (Session, error)
}

// DuplicateRepository defines persistence operations for duplicate tracking.
type DuplicateRepository interface {
	Create(ctx context.Context, duplicate Duplicate) (Duplicate, error)
	ListByObservationID(ctx context.Context, observationID string) ([]Duplicate, error)
}

// SearchIndex defines full-text search operations.
type SearchIndex interface {
	Index(ctx context.Context, observation Observation) error
	Remove(ctx context.Context, observationID string) error
	Search(ctx context.Context, query string, filter SearchFilter) ([]SearchResult, error)
}

// Clock supplies timestamps for deterministic operations.
type Clock interface {
	Now() time.Time
}

// PathResolver resolves storage paths based on configuration sources.
type PathResolver interface {
	Resolve(ctx context.Context, overrides ConfigOverrides) (ResolvedPaths, error)
}

// ConfigLoader loads effective configuration with precedence rules.
type ConfigLoader interface {
	Load(ctx context.Context, overrides ConfigOverrides) (Config, error)
}

// SearchResult represents a search match with score metadata.
type SearchResult struct {
	ObservationID string
	Score         float64
}

// ConfigOverrides provides adapter-level configuration overrides.
type ConfigOverrides struct {
	StoragePath    *string
	FTSPath        *string
	DefaultProject *string
	DedupePolicy   *string
	ConfigFile     *string
}

// ResolvedPaths holds resolved storage paths.
type ResolvedPaths struct {
	StoragePath string
	FTSPath     string
}

// Config represents the effective configuration used by the app.
type Config struct {
	StoragePath    string
	FTSPath        string
	DefaultProject string
	DedupePolicy   string
}
