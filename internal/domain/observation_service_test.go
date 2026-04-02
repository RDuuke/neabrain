package domain_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"neabrain/internal/domain"
)

func TestObservationServiceCreateRejectsDuplicate(t *testing.T) {
	repo := newInMemoryObservationRepo()
	duplicateRepo := &inMemoryDuplicateRepo{}
	search := &stubSearchIndex{}
	clock := stubClock{now: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)}

	_, _ = repo.Create(context.Background(), domain.Observation{
		ID:        "obs-1",
		Content:   "Hello",
		CreatedAt: clock.now,
		UpdatedAt: clock.now,
		Project:   "alpha",
	})

	svc := domain.NewObservationService(repo, search, clock, duplicateRepo, domain.ExactMatchDedupePolicy{})

	_, err := svc.Create(context.Background(), domain.ObservationCreateInput{Content: "Hello", Project: "alpha"})
	if err == nil {
		t.Fatal("expected duplicate conflict")
	}
	var domainErr domain.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != domain.ErrorConflict {
		t.Fatalf("expected conflict error, got %v", err)
	}
	if len(duplicateRepo.items) != 0 {
		t.Fatalf("expected no duplicate records, got %d", len(duplicateRepo.items))
	}
}

func TestObservationServiceCreateAllowsDuplicateWithOverride(t *testing.T) {
	repo := newInMemoryObservationRepo()
	duplicateRepo := &inMemoryDuplicateRepo{}
	search := &stubSearchIndex{}
	clock := stubClock{now: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)}

	_, _ = repo.Create(context.Background(), domain.Observation{
		ID:        "obs-1",
		Content:   "Hello",
		CreatedAt: clock.now,
		UpdatedAt: clock.now,
		Project:   "alpha",
	})

	svc := domain.NewObservationService(repo, search, clock, duplicateRepo, domain.ExactMatchDedupePolicy{})

	created, err := svc.Create(context.Background(), domain.ObservationCreateInput{Content: "Hello", Project: "alpha", AllowDuplicate: true})
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created observation id")
	}
	if len(duplicateRepo.items) != 1 {
		t.Fatalf("expected 1 duplicate record, got %d", len(duplicateRepo.items))
	}
	if duplicateRepo.items[0].OriginalObservationID != "obs-1" {
		t.Fatalf("expected original obs-1, got %s", duplicateRepo.items[0].OriginalObservationID)
	}
}

func TestObservationServiceUpdateRejectsDuplicate(t *testing.T) {
	repo := newInMemoryObservationRepo()
	duplicateRepo := &inMemoryDuplicateRepo{}
	search := &stubSearchIndex{}
	clock := stubClock{now: time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC)}

	_, _ = repo.Create(context.Background(), domain.Observation{
		ID:        "obs-1",
		Content:   "Hello",
		CreatedAt: clock.now,
		UpdatedAt: clock.now,
		Project:   "alpha",
	})
	_, _ = repo.Create(context.Background(), domain.Observation{
		ID:        "obs-2",
		Content:   "World",
		CreatedAt: clock.now,
		UpdatedAt: clock.now,
		Project:   "alpha",
	})

	svc := domain.NewObservationService(repo, search, clock, duplicateRepo, domain.ExactMatchDedupePolicy{})

	content := "Hello"
	_, err := svc.Update(context.Background(), domain.ObservationUpdateInput{ID: "obs-2", Content: &content})
	if err == nil {
		t.Fatal("expected duplicate conflict")
	}

	loaded, err := repo.GetByID(context.Background(), "obs-2", true)
	if err != nil {
		t.Fatalf("expected observation to exist, got %v", err)
	}
	if loaded.Content != "World" {
		t.Fatalf("expected content to remain unchanged, got %s", loaded.Content)
	}
}

func TestObservationServiceSoftDeleteUsesClock(t *testing.T) {
	repo := newInMemoryObservationRepo()
	duplicateRepo := &inMemoryDuplicateRepo{}
	search := &stubSearchIndex{}
	clock := stubClock{now: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)}

	_, _ = repo.Create(context.Background(), domain.Observation{
		ID:        "obs-1",
		Content:   "Hello",
		CreatedAt: clock.now,
		UpdatedAt: clock.now,
		Project:   "alpha",
	})

	svc := domain.NewObservationService(repo, search, clock, duplicateRepo, domain.ExactMatchDedupePolicy{})

	deleted, err := svc.SoftDelete(context.Background(), "obs-1")
	if err != nil {
		t.Fatalf("expected soft delete to succeed, got %v", err)
	}
	if deleted.DeletedAt == nil || !deleted.DeletedAt.Equal(clock.now) {
		t.Fatalf("expected deleted_at to be %v, got %#v", clock.now, deleted.DeletedAt)
	}
}

func TestSessionServiceResumeUpdatesLastSeen(t *testing.T) {
	repo := newInMemorySessionRepo()
	clock := stubClock{now: time.Date(2026, 5, 5, 13, 0, 0, 0, time.UTC)}

	_, _ = repo.Create(context.Background(), domain.Session{
		ID:              "session-1",
		CreatedAt:       clock.now.Add(-time.Hour),
		LastSeenAt:      clock.now.Add(-time.Hour),
		DisclosureLevel: "low",
	})

	svc := domain.NewSessionService(repo, clock)

	resumed, err := svc.Resume(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("expected resume to succeed, got %v", err)
	}
	if !resumed.LastSeenAt.Equal(clock.now) {
		t.Fatalf("expected last_seen_at to be %v, got %v", clock.now, resumed.LastSeenAt)
	}
}

func TestObservationServiceListFiltersByDisclosureLevel(t *testing.T) {
	repo := newInMemoryObservationRepo()
	search := &stubSearchIndex{}
	clock := stubClock{now: time.Date(2026, 5, 6, 13, 0, 0, 0, time.UTC)}

	for _, observation := range []domain.Observation{
		{ID: "obs-public", Content: "public", CreatedAt: clock.now, UpdatedAt: clock.now, Tags: []string{"shared"}},
		{ID: "obs-private", Content: "private", CreatedAt: clock.now, UpdatedAt: clock.now, Tags: []string{"private"}},
		{ID: "obs-sensitive", Content: "sensitive", CreatedAt: clock.now, UpdatedAt: clock.now, Tags: []string{"sensitive"}},
	} {
		if _, err := repo.Create(context.Background(), observation); err != nil {
			t.Fatalf("seed observation %s: %v", observation.ID, err)
		}
	}

	svc := domain.NewObservationService(repo, search, clock, &inMemoryDuplicateRepo{}, domain.ExactMatchDedupePolicy{})

	lowResults, err := svc.List(context.Background(), domain.ObservationListFilter{DisclosureLevel: "low"})
	if err != nil {
		t.Fatalf("list low: %v", err)
	}
	if len(lowResults) != 1 || lowResults[0].ID != "obs-public" {
		t.Fatalf("unexpected low disclosure results: %#v", lowResults)
	}

	mediumResults, err := svc.List(context.Background(), domain.ObservationListFilter{DisclosureLevel: "medium"})
	if err != nil {
		t.Fatalf("list medium: %v", err)
	}
	if len(mediumResults) != 2 {
		t.Fatalf("expected 2 medium results, got %d", len(mediumResults))
	}
}

func TestSearchServiceFiltersByDisclosureLevel(t *testing.T) {
	repo := newInMemoryObservationRepo()
	clock := stubClock{now: time.Date(2026, 5, 7, 13, 0, 0, 0, time.UTC)}
	for _, observation := range []domain.Observation{
		{ID: "obs-public", Content: "public", CreatedAt: clock.now, UpdatedAt: clock.now, Tags: []string{"shared"}},
		{ID: "obs-private", Content: "private", CreatedAt: clock.now, UpdatedAt: clock.now, Tags: []string{"private"}},
		{ID: "obs-sensitive", Content: "sensitive", CreatedAt: clock.now, UpdatedAt: clock.now, Tags: []string{"sensitive"}},
	} {
		if _, err := repo.Create(context.Background(), observation); err != nil {
			t.Fatalf("seed observation %s: %v", observation.ID, err)
		}
	}

	search := &stubSearchIndex{results: []domain.SearchResult{
		{ObservationID: "obs-public"},
		{ObservationID: "obs-private"},
		{ObservationID: "obs-sensitive"},
	}}
	svc := domain.NewSearchService(repo, search)

	results, err := svc.Search(context.Background(), "anything", domain.SearchFilter{DisclosureLevel: "low"})
	if err != nil {
		t.Fatalf("search low disclosure: %v", err)
	}
	if len(results) != 1 || results[0].ID != "obs-public" {
		t.Fatalf("unexpected low disclosure search results: %#v", results)
	}
}

func TestObservationServiceTimelineValidatesInputs(t *testing.T) {
	repo := newInMemoryObservationRepo()
	search := &stubSearchIndex{}
	clock := stubClock{now: time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC)}
	svc := domain.NewObservationService(repo, search, clock, &inMemoryDuplicateRepo{}, domain.ExactMatchDedupePolicy{})

	if _, err := svc.Timeline(context.Background(), "", 1, 1, false); err == nil {
		t.Fatal("expected missing id error")
	}
	if _, err := svc.Timeline(context.Background(), "obs-1", -1, 1, false); err == nil {
		t.Fatal("expected invalid before error")
	}
	if _, err := svc.Timeline(context.Background(), "obs-1", 1, -1, false); err == nil {
		t.Fatal("expected invalid after error")
	}
}

func TestObservationServiceTimeline(t *testing.T) {
	repo := newInMemoryObservationRepo()
	search := &stubSearchIndex{}
	clock := stubClock{now: time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC)}

	timeline := []domain.Observation{
		{ID: "obs-1", Content: "one", CreatedAt: clock.now.Add(-3 * time.Minute), UpdatedAt: clock.now},
		{ID: "obs-2", Content: "two", CreatedAt: clock.now.Add(-2 * time.Minute), UpdatedAt: clock.now},
		{ID: "obs-3", Content: "three", CreatedAt: clock.now.Add(-time.Minute), UpdatedAt: clock.now},
	}
	for _, observation := range timeline {
		if _, err := repo.Create(context.Background(), observation); err != nil {
			t.Fatalf("seed observation %s: %v", observation.ID, err)
		}
	}

	svc := domain.NewObservationService(repo, search, clock, &inMemoryDuplicateRepo{}, domain.ExactMatchDedupePolicy{})
	result, err := svc.Timeline(context.Background(), "obs-2", 1, 1, false)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	if result.Target.ID != "obs-2" {
		t.Fatalf("expected target obs-2, got %#v", result.Target)
	}
	if len(result.Before) != 1 || result.Before[0].ID != "obs-1" {
		t.Fatalf("unexpected before results: %#v", result.Before)
	}
	if len(result.After) != 1 || result.After[0].ID != "obs-3" {
		t.Fatalf("unexpected after results: %#v", result.After)
	}
}

type stubClock struct {
	now time.Time
}

func (c stubClock) Now() time.Time {
	return c.now
}

type stubSearchIndex struct {
	indexed []domain.Observation
	err     error
	results []domain.SearchResult
}

func (s *stubSearchIndex) Index(ctx context.Context, observation domain.Observation) error {
	s.indexed = append(s.indexed, observation)
	return s.err
}

func (s *stubSearchIndex) Remove(ctx context.Context, observationID string) error {
	return s.err
}

func (s *stubSearchIndex) Search(ctx context.Context, query string, filter domain.SearchFilter) ([]domain.SearchResult, error) {
	return s.results, s.err
}

type inMemoryObservationRepo struct {
	items map[string]domain.Observation
}

func newInMemoryObservationRepo() *inMemoryObservationRepo {
	return &inMemoryObservationRepo{items: map[string]domain.Observation{}}
}

func (r *inMemoryObservationRepo) Create(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	if observation.ID == "" {
		return domain.Observation{}, domain.NewInvalidInput("observation id is required")
	}
	if _, exists := r.items[observation.ID]; exists {
		return domain.Observation{}, domain.NewConflict("observation already exists")
	}
	r.items[observation.ID] = observation
	return observation, nil
}

func (r *inMemoryObservationRepo) GetByID(ctx context.Context, id string, includeDeleted bool) (domain.Observation, error) {
	observation, exists := r.items[id]
	if !exists || (!includeDeleted && observation.DeletedAt != nil) {
		return domain.Observation{}, domain.NewNotFound("observation not found")
	}
	return observation, nil
}

func (r *inMemoryObservationRepo) Update(ctx context.Context, observation domain.Observation) (domain.Observation, error) {
	if _, exists := r.items[observation.ID]; !exists {
		return domain.Observation{}, domain.NewNotFound("observation not found")
	}
	r.items[observation.ID] = observation
	return observation, nil
}

func (r *inMemoryObservationRepo) List(ctx context.Context, filter domain.ObservationListFilter) ([]domain.Observation, error) {
	results := make([]domain.Observation, 0)
	for _, observation := range r.items {
		if filter.Project != "" && observation.Project != filter.Project {
			continue
		}
		if filter.TopicKey != "" && observation.TopicKey != filter.TopicKey {
			continue
		}
		if !filter.IncludeDeleted && observation.DeletedAt != nil {
			continue
		}
		if len(filter.Tags) > 0 && !testContainsAllTags(observation.Tags, filter.Tags) {
			continue
		}
		results = append(results, observation)
	}
	return results, nil
}

func (r *inMemoryObservationRepo) FindAround(ctx context.Context, id string, before, after int, includeDeleted bool) (domain.TimelineResult, error) {
	target, err := r.GetByID(ctx, id, includeDeleted)
	if err != nil {
		return domain.TimelineResult{}, err
	}

	ordered := make([]domain.Observation, 0, len(r.items))
	for _, observation := range r.items {
		if !includeDeleted && observation.DeletedAt != nil {
			continue
		}
		ordered = append(ordered, observation)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})

	index := -1
	for i, observation := range ordered {
		if observation.ID == id {
			index = i
			break
		}
	}
	if index == -1 {
		return domain.TimelineResult{}, domain.NewNotFound("observation not found")
	}

	start := index - before
	if start < 0 {
		start = 0
	}
	end := index + after + 1
	if end > len(ordered) {
		end = len(ordered)
	}

	result := domain.TimelineResult{Target: target}
	result.Before = append(result.Before, ordered[start:index]...)
	result.After = append(result.After, ordered[index+1:end]...)
	return result, nil
}

func (r *inMemoryObservationRepo) SoftDelete(ctx context.Context, id string, deletedAt time.Time) (domain.Observation, error) {
	observation, exists := r.items[id]
	if !exists {
		return domain.Observation{}, domain.NewNotFound("observation not found")
	}
	observation.DeletedAt = &deletedAt
	observation.UpdatedAt = deletedAt
	r.items[id] = observation
	return observation, nil
}

func (r *inMemoryObservationRepo) FindByContent(ctx context.Context, content string, project string, includeDeleted bool) ([]domain.Observation, error) {
	results := make([]domain.Observation, 0)
	for _, observation := range r.items {
		if observation.Content != content || observation.Project != project {
			continue
		}
		if !includeDeleted && observation.DeletedAt != nil {
			continue
		}
		results = append(results, observation)
	}
	return results, nil
}

func (r *inMemoryObservationRepo) GetStats(ctx context.Context) (domain.ObservationStats, error) {
	var stats domain.ObservationStats
	for _, obs := range r.items {
		if obs.DeletedAt == nil {
			stats.Active++
		} else {
			stats.Deleted++
		}
	}
	projects := map[string]struct{}{}
	for _, obs := range r.items {
		if obs.DeletedAt == nil && obs.Project != "" {
			projects[obs.Project] = struct{}{}
		}
	}
	stats.Projects = len(projects)
	return stats, nil
}

func (r *inMemoryObservationRepo) ListProjects(ctx context.Context) ([]domain.ProjectSummary, error) {
	counts := map[string]int{}
	for _, obs := range r.items {
		if obs.DeletedAt == nil && obs.Project != "" {
			counts[obs.Project]++
		}
	}
	summaries := make([]domain.ProjectSummary, 0, len(counts))
	for name, count := range counts {
		summaries = append(summaries, domain.ProjectSummary{Name: name, Count: count})
	}
	return summaries, nil
}

func (r *inMemoryObservationRepo) RenameProject(ctx context.Context, oldName, newName string) (int, error) {
	count := 0
	for id, obs := range r.items {
		if obs.Project == oldName {
			obs.Project = newName
			r.items[id] = obs
			count++
		}
	}
	return count, nil
}

type inMemoryDuplicateRepo struct {
	items []domain.Duplicate
}

func (r *inMemoryDuplicateRepo) Create(ctx context.Context, duplicate domain.Duplicate) (domain.Duplicate, error) {
	if duplicate.ID == "" {
		return domain.Duplicate{}, domain.NewInvalidInput("duplicate id is required")
	}
	r.items = append(r.items, duplicate)
	return duplicate, nil
}

func (r *inMemoryDuplicateRepo) ListByObservationID(ctx context.Context, observationID string) ([]domain.Duplicate, error) {
	if observationID == "" {
		return nil, domain.NewInvalidInput("observation id is required")
	}
	results := make([]domain.Duplicate, 0)
	for _, duplicate := range r.items {
		if duplicate.OriginalObservationID == observationID || duplicate.DuplicateObservationID == observationID {
			results = append(results, duplicate)
		}
	}
	return results, nil
}

type inMemorySessionRepo struct {
	items map[string]domain.Session
}

func newInMemorySessionRepo() *inMemorySessionRepo {
	return &inMemorySessionRepo{items: map[string]domain.Session{}}
}

func (r *inMemorySessionRepo) Create(ctx context.Context, session domain.Session) (domain.Session, error) {
	if session.ID == "" {
		return domain.Session{}, domain.NewInvalidInput("session id is required")
	}
	if _, exists := r.items[session.ID]; exists {
		return domain.Session{}, domain.NewConflict("session already exists")
	}
	r.items[session.ID] = session
	return session, nil
}

func (r *inMemorySessionRepo) GetByID(ctx context.Context, id string) (domain.Session, error) {
	session, exists := r.items[id]
	if !exists {
		return domain.Session{}, domain.NewNotFound("session not found")
	}
	return session, nil
}

func (r *inMemorySessionRepo) Update(ctx context.Context, session domain.Session) (domain.Session, error) {
	if _, exists := r.items[session.ID]; !exists {
		return domain.Session{}, domain.NewNotFound("session not found")
	}
	r.items[session.ID] = session
	return session, nil
}

func testContainsAllTags(haystack []string, needles []string) bool {
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
