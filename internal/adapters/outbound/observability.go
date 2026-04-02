package outbound

import (
	"context"
	"time"

	"neabrain/internal/domain"
	"neabrain/internal/observability"
	ports "neabrain/internal/ports/outbound"
)

// WrapObservationRepository adds logging and metrics.
func WrapObservationRepository(next ports.ObservationRepository, logger *observability.Logger, metrics *observability.Metrics) ports.ObservationRepository {
	if next == nil {
		return nil
	}
	return &observedObservationRepository{next: next, logger: logger, metrics: metrics}
}

// WrapTopicRepository adds logging and metrics.
func WrapTopicRepository(next ports.TopicRepository, logger *observability.Logger, metrics *observability.Metrics) ports.TopicRepository {
	if next == nil {
		return nil
	}
	return &observedTopicRepository{next: next, logger: logger, metrics: metrics}
}

// WrapSessionRepository adds logging and metrics.
func WrapSessionRepository(next ports.SessionRepository, logger *observability.Logger, metrics *observability.Metrics) ports.SessionRepository {
	if next == nil {
		return nil
	}
	return &observedSessionRepository{next: next, logger: logger, metrics: metrics}
}

// WrapDuplicateRepository adds logging and metrics.
func WrapDuplicateRepository(next ports.DuplicateRepository, logger *observability.Logger, metrics *observability.Metrics) ports.DuplicateRepository {
	if next == nil {
		return nil
	}
	return &observedDuplicateRepository{next: next, logger: logger, metrics: metrics}
}

// WrapSearchIndex adds logging and metrics.
func WrapSearchIndex(next ports.SearchIndex, logger *observability.Logger, metrics *observability.Metrics) ports.SearchIndex {
	if next == nil {
		return nil
	}
	return &observedSearchIndex{next: next, logger: logger, metrics: metrics}
}

type observedObservationRepository struct {
	next    ports.ObservationRepository
	logger  *observability.Logger
	metrics *observability.Metrics
}

func (o *observedObservationRepository) Create(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	o.metrics.Inc("repo.observation.create")
	o.logger.Info("repo observation create", map[string]any{"id": observation.ID})
	return o.next.Create(ctx, observation)
}

func (o *observedObservationRepository) GetByID(ctx context.Context, id string, includeDeleted bool) (domain.Observation, error) {
	o.metrics.Inc("repo.observation.get")
	o.logger.Info("repo observation get", map[string]any{"id": id, "include_deleted": includeDeleted})
	return o.next.GetByID(ctx, id, includeDeleted)
}

func (o *observedObservationRepository) Update(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	o.metrics.Inc("repo.observation.update")
	o.logger.Info("repo observation update", map[string]any{"id": observation.ID})
	return o.next.Update(ctx, observation)
}

func (o *observedObservationRepository) List(ctx context.Context, filter ports.ObservationFilter) ([]domain.Observation, error) {
	o.metrics.Inc("repo.observation.list")
	o.logger.Info("repo observation list", map[string]any{"project": filter.Project, "topic_key": filter.TopicKey, "include_deleted": filter.IncludeDeleted})
	return o.next.List(ctx, filter)
}

func (o *observedObservationRepository) FindAround(ctx context.Context, id string, before, after int, includeDeleted bool) (domain.TimelineResult, error) {
	o.metrics.Inc("repo.observation.timeline")
	o.logger.Info("repo observation timeline", map[string]any{"id": id, "before": before, "after": after, "include_deleted": includeDeleted})
	return o.next.FindAround(ctx, id, before, after, includeDeleted)
}

func (o *observedObservationRepository) SoftDelete(ctx context.Context, id string, deletedAt time.Time) (domain.Observation, error) {
	o.metrics.Inc("repo.observation.delete")
	o.logger.Info("repo observation delete", map[string]any{"id": id})
	return o.next.SoftDelete(ctx, id, deletedAt)
}

func (o *observedObservationRepository) GetStats(ctx context.Context) (domain.ObservationStats, error) {
	o.metrics.Inc("repo.observation.stats")
	o.logger.Info("repo observation stats", nil)
	return o.next.GetStats(ctx)
}

func (o *observedObservationRepository) ListProjects(ctx context.Context) ([]domain.ProjectSummary, error) {
	o.metrics.Inc("repo.observation.list_projects")
	o.logger.Info("repo observation list projects", nil)
	return o.next.ListProjects(ctx)
}

func (o *observedObservationRepository) RenameProject(ctx context.Context, oldName, newName string) (int, error) {
	o.metrics.Inc("repo.observation.rename_project")
	o.logger.Info("repo observation rename project", map[string]any{"old": oldName, "new": newName})
	return o.next.RenameProject(ctx, oldName, newName)
}

func (o *observedObservationRepository) FindByContent(ctx context.Context, content string, project string, includeDeleted bool) ([]domain.Observation, error) {
	o.metrics.Inc("repo.observation.find_by_content")
	o.logger.Info("repo observation find by content", map[string]any{"project": project, "include_deleted": includeDeleted, "content_length": len(content)})
	return o.next.FindByContent(ctx, content, project, includeDeleted)
}

type observedTopicRepository struct {
	next    ports.TopicRepository
	logger  *observability.Logger
	metrics *observability.Metrics
}

func (o *observedTopicRepository) UpsertByTopicKey(ctx context.Context, topic domain.Topic) (domain.Topic, error) {
	o.metrics.Inc("repo.topic.upsert")
	o.logger.Info("repo topic upsert", map[string]any{"topic_key": topic.TopicKey})
	return o.next.UpsertByTopicKey(ctx, topic)
}

func (o *observedTopicRepository) GetByTopicKey(ctx context.Context, topicKey string) (domain.Topic, error) {
	o.metrics.Inc("repo.topic.get")
	o.logger.Info("repo topic get", map[string]any{"topic_key": topicKey})
	return o.next.GetByTopicKey(ctx, topicKey)
}

func (o *observedTopicRepository) List(ctx context.Context) ([]domain.TopicSummary, error) {
	o.metrics.Inc("repo.topic.list")
	o.logger.Info("repo topic list", nil)
	return o.next.List(ctx)
}

type observedSessionRepository struct {
	next    ports.SessionRepository
	logger  *observability.Logger
	metrics *observability.Metrics
}

func (o *observedSessionRepository) Create(ctx context.Context, session domain.Session) (domain.Session, error) {
	o.metrics.Inc("repo.session.create")
	o.logger.Info("repo session create", map[string]any{"id": session.ID})
	return o.next.Create(ctx, session)
}

func (o *observedSessionRepository) GetByID(ctx context.Context, id string) (domain.Session, error) {
	o.metrics.Inc("repo.session.get")
	o.logger.Info("repo session get", map[string]any{"id": id})
	return o.next.GetByID(ctx, id)
}

func (o *observedSessionRepository) Update(ctx context.Context, session domain.Session) (domain.Session, error) {
	o.metrics.Inc("repo.session.update")
	o.logger.Info("repo session update", map[string]any{"id": session.ID})
	return o.next.Update(ctx, session)
}

type observedDuplicateRepository struct {
	next    ports.DuplicateRepository
	logger  *observability.Logger
	metrics *observability.Metrics
}

func (o *observedDuplicateRepository) Create(ctx context.Context, duplicate domain.Duplicate) (domain.Duplicate, error) {
	o.metrics.Inc("repo.duplicate.create")
	o.logger.Info("repo duplicate create", map[string]any{"original_id": duplicate.OriginalObservationID, "duplicate_id": duplicate.DuplicateObservationID})
	return o.next.Create(ctx, duplicate)
}

func (o *observedDuplicateRepository) ListByObservationID(ctx context.Context, observationID string) ([]domain.Duplicate, error) {
	o.metrics.Inc("repo.duplicate.list")
	o.logger.Info("repo duplicate list", map[string]any{"observation_id": observationID})
	return o.next.ListByObservationID(ctx, observationID)
}

type observedSearchIndex struct {
	next    ports.SearchIndex
	logger  *observability.Logger
	metrics *observability.Metrics
}

func (o *observedSearchIndex) Index(ctx context.Context, observation domain.Observation) error {
	o.metrics.Inc("repo.search.index")
	o.logger.Info("repo search index", map[string]any{"id": observation.ID})
	return o.next.Index(ctx, observation)
}

func (o *observedSearchIndex) Remove(ctx context.Context, observationID string) error {
	o.metrics.Inc("repo.search.remove")
	o.logger.Info("repo search remove", map[string]any{"id": observationID})
	return o.next.Remove(ctx, observationID)
}

func (o *observedSearchIndex) Search(ctx context.Context, query string, filter ports.SearchFilter) ([]ports.SearchResult, error) {
	o.metrics.Inc("repo.search.query")
	o.logger.Info("repo search query", map[string]any{"project": filter.Project, "topic_key": filter.TopicKey, "include_deleted": filter.IncludeDeleted, "query_length": len(query)})
	return o.next.Search(ctx, query, filter)
}
