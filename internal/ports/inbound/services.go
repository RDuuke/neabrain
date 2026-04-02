package inbound

import (
	"context"

	"neabrain/internal/domain"
)

// ObservationService defines core observation use cases.
type ObservationService interface {
	Create(ctx context.Context, input ObservationCreateInput) (domain.Observation, error)
	Read(ctx context.Context, id string, includeDeleted bool) (domain.Observation, error)
	Update(ctx context.Context, input ObservationUpdateInput) (domain.Observation, error)
	List(ctx context.Context, filter ObservationListFilter) ([]domain.Observation, error)
	Timeline(ctx context.Context, id string, before, after int, includeDeleted bool) (domain.TimelineResult, error)
	SoftDelete(ctx context.Context, id string) (domain.Observation, error)
}

// SearchService defines full-text search use cases.
type SearchService interface {
	Search(ctx context.Context, query string, filter SearchFilter) ([]domain.Observation, error)
}

// TopicService defines topic management use cases.
type TopicService interface {
	UpsertByTopicKey(ctx context.Context, input TopicUpsertInput) (domain.Topic, error)
	List(ctx context.Context) ([]domain.TopicSummary, error)
}

// SessionService defines session lifecycle use cases.
type SessionService interface {
	Open(ctx context.Context, input SessionOpenInput) (domain.Session, error)
	Read(ctx context.Context, id string) (domain.Session, error)
	Resume(ctx context.Context, id string) (domain.Session, error)
	UpdateDisclosure(ctx context.Context, id string, disclosureLevel string) (domain.Session, error)
}

// ConfigService defines configuration resolution use cases.
type ConfigService interface {
	ResolvePaths(ctx context.Context, overrides ConfigOverrides) (ResolvedPaths, error)
	LoadConfig(ctx context.Context, overrides ConfigOverrides) (Config, error)
}

// ObservationCreateInput captures fields required to create an observation.
type ObservationCreateInput struct {
	Content        string
	Project        string
	TopicKey       string
	Tags           []string
	Source         string
	Metadata       map[string]any
	AllowDuplicate bool
}

// ObservationUpdateInput captures fields required to update an observation.
type ObservationUpdateInput struct {
	ID       string
	Content  *string
	Project  *string
	TopicKey *string
	Tags     []string
	Source   *string
	Metadata map[string]any
}

// ObservationListFilter constrains observation listings.
type ObservationListFilter struct {
	Project        string
	TopicKey       string
	Tags           []string
	IncludeDeleted bool
}

// SearchFilter constrains search results.
type SearchFilter struct {
	Project        string
	TopicKey       string
	Tags           []string
	IncludeDeleted bool
}

// TopicUpsertInput captures fields required to upsert a topic.
type TopicUpsertInput struct {
	TopicKey    string
	Name        string
	Description string
	Metadata    map[string]any
}

// SessionOpenInput captures fields required to open a session.
type SessionOpenInput struct {
	DisclosureLevel string
}

// ConfigOverrides provides adapter-level configuration overrides.
type ConfigOverrides = domain.ConfigOverrides

// ResolvedPaths holds resolved storage paths.
type ResolvedPaths = domain.ResolvedPaths

// Config represents the effective configuration used by the app.
type Config = domain.Config
