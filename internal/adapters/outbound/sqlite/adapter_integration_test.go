package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"neabrain/internal/domain"
	sqliteinfra "neabrain/internal/infrastructure/sqlite"
	ports "neabrain/internal/ports/outbound"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "neabrain.db")
	conn, err := sqliteinfra.Open(ctx, sqliteinfra.Config{Path: path})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := sqliteinfra.ApplyMigrations(ctx, conn); err != nil {
		_ = conn.Close()
		t.Fatalf("apply migrations: %v", err)
	}

	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

func TestObservationRepositoryCRUD(t *testing.T) {
	db := openTestDB(t)
	repo := NewObservationRepository(db)
	ctx := context.Background()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	observation := domain.Observation{
		ID:        "obs-1",
		Content:   "Hello world",
		CreatedAt: now,
		UpdatedAt: now,
		Project:   "alpha",
		TopicKey:  "topic-1",
		Tags:      []string{"a", "b"},
		Source:    "cli",
		Metadata:  map[string]any{"origin": "test"},
	}

	created, err := repo.Create(ctx, observation)
	if err != nil {
		t.Fatalf("create observation: %v", err)
	}
	if created.ID != observation.ID {
		t.Fatalf("expected id %s, got %s", observation.ID, created.ID)
	}

	loaded, err := repo.GetByID(ctx, observation.ID, false)
	if err != nil {
		t.Fatalf("get observation: %v", err)
	}
	if loaded.Content != observation.Content {
		t.Fatalf("expected content %q, got %q", observation.Content, loaded.Content)
	}
	if !reflect.DeepEqual(loaded.Metadata, observation.Metadata) {
		t.Fatalf("expected metadata %#v, got %#v", observation.Metadata, loaded.Metadata)
	}

	updated := loaded
	updated.Content = "Updated"
	updated.UpdatedAt = now.Add(2 * time.Hour)
	updated.Tags = []string{"a"}
	if _, err := repo.Update(ctx, updated); err != nil {
		t.Fatalf("update observation: %v", err)
	}

	listed, err := repo.List(ctx, ports.ObservationFilter{Project: "alpha", Tags: []string{"a"}})
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(listed))
	}

	deletedAt := now.Add(3 * time.Hour)
	if _, err := repo.SoftDelete(ctx, observation.ID, deletedAt); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	_, err = repo.GetByID(ctx, observation.ID, false)
	if err == nil {
		t.Fatal("expected not found after soft delete")
	}
	var domainErr domain.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != domain.ErrorNotFound {
		t.Fatalf("expected not found error, got %v", err)
	}

	listed, err = repo.List(ctx, ports.ObservationFilter{IncludeDeleted: false})
	if err != nil {
		t.Fatalf("list without deleted: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected 0 observations, got %d", len(listed))
	}

	listed, err = repo.List(ctx, ports.ObservationFilter{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("list with deleted: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(listed))
	}
}

func TestObservationRepositoryFindAround(t *testing.T) {
	db := openTestDB(t)
	repo := NewObservationRepository(db)
	ctx := context.Background()

	base := time.Date(2026, 1, 3, 3, 4, 5, 0, time.UTC)
	observations := []domain.Observation{
		{ID: "obs-1", Content: "first", CreatedAt: base.Add(-3 * time.Minute), UpdatedAt: base},
		{ID: "obs-2", Content: "second", CreatedAt: base.Add(-2 * time.Minute), UpdatedAt: base},
		{ID: "obs-3", Content: "third", CreatedAt: base.Add(-2 * time.Minute), UpdatedAt: base},
		{ID: "obs-4", Content: "fourth", CreatedAt: base.Add(-time.Minute), UpdatedAt: base},
	}
	for _, observation := range observations {
		if _, err := repo.Create(ctx, observation); err != nil {
			t.Fatalf("create observation %s: %v", observation.ID, err)
		}
	}

	result, err := repo.FindAround(ctx, "obs-3", 2, 1, false)
	if err != nil {
		t.Fatalf("find around: %v", err)
	}
	if result.Target.ID != "obs-3" {
		t.Fatalf("expected target obs-3, got %#v", result.Target)
	}
	if len(result.Before) != 2 || result.Before[0].ID != "obs-1" || result.Before[1].ID != "obs-2" {
		t.Fatalf("unexpected before: %#v", result.Before)
	}
	if len(result.After) != 1 || result.After[0].ID != "obs-4" {
		t.Fatalf("unexpected after: %#v", result.After)
	}
}

func TestTopicRepositoryUpsert(t *testing.T) {
	db := openTestDB(t)
	repo := NewTopicRepository(db)
	ctx := context.Background()

	now := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	topic := domain.Topic{
		ID:          "topic-1",
		TopicKey:    "t-k",
		Name:        "Original",
		Description: "first",
		Metadata:    map[string]any{"k": "v"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := repo.UpsertByTopicKey(ctx, topic)
	if err != nil {
		t.Fatalf("insert topic: %v", err)
	}

	updated := topic
	updated.Name = "Updated"
	updated.Description = "second"
	updated.UpdatedAt = now.Add(time.Hour)
	updated.ID = "ignored"

	upserted, err := repo.UpsertByTopicKey(ctx, updated)
	if err != nil {
		t.Fatalf("update topic: %v", err)
	}
	if upserted.ID != created.ID {
		t.Fatalf("expected id %s, got %s", created.ID, upserted.ID)
	}

	loaded, err := repo.GetByTopicKey(ctx, topic.TopicKey)
	if err != nil {
		t.Fatalf("get topic: %v", err)
	}
	if loaded.Name != updated.Name {
		t.Fatalf("expected name %q, got %q", updated.Name, loaded.Name)
	}
}

func TestTopicRepositoryList(t *testing.T) {
	db := openTestDB(t)
	topicRepo := NewTopicRepository(db)
	observationRepo := NewObservationRepository(db)
	ctx := context.Background()

	now := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	for _, topic := range []domain.Topic{
		{
			ID:          "topic-1",
			TopicKey:    "alpha-topic",
			Name:        "Alpha",
			Description: "first topic",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "topic-2",
			TopicKey:    "beta-topic",
			Name:        "Beta",
			Description: "second topic",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	} {
		if _, err := topicRepo.UpsertByTopicKey(ctx, topic); err != nil {
			t.Fatalf("upsert topic %s: %v", topic.TopicKey, err)
		}
	}

	activeObservation := domain.Observation{
		ID:        "obs-1",
		Content:   "active",
		CreatedAt: now,
		UpdatedAt: now,
		Project:   "alpha",
		TopicKey:  "alpha-topic",
	}
	if _, err := observationRepo.Create(ctx, activeObservation); err != nil {
		t.Fatalf("create active observation: %v", err)
	}

	deletedObservation := domain.Observation{
		ID:        "obs-2",
		Content:   "deleted",
		CreatedAt: now,
		UpdatedAt: now,
		Project:   "alpha",
		TopicKey:  "beta-topic",
	}
	if _, err := observationRepo.Create(ctx, deletedObservation); err != nil {
		t.Fatalf("create deleted observation: %v", err)
	}
	if _, err := observationRepo.SoftDelete(ctx, deletedObservation.ID, now.Add(time.Hour)); err != nil {
		t.Fatalf("soft delete observation: %v", err)
	}

	summaries, err := topicRepo.List(ctx)
	if err != nil {
		t.Fatalf("list topics: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 topic summaries, got %d", len(summaries))
	}
	if summaries[0].TopicKey != "alpha-topic" || summaries[0].Count != 1 {
		t.Fatalf("unexpected first summary: %#v", summaries[0])
	}
	if summaries[1].TopicKey != "beta-topic" || summaries[1].Count != 0 {
		t.Fatalf("unexpected second summary: %#v", summaries[1])
	}
}

func TestSessionRepositoryCRUD(t *testing.T) {
	db := openTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	session := domain.Session{
		ID:               "session-1",
		CreatedAt:        now,
		LastSeenAt:       now,
		DisclosureLevel:  "low",
		RecentOperations: []string{"open"},
	}

	created, err := repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if created.ID != session.ID {
		t.Fatalf("expected id %s, got %s", session.ID, created.ID)
	}

	loaded, err := repo.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if loaded.DisclosureLevel != session.DisclosureLevel {
		t.Fatalf("expected disclosure %q, got %q", session.DisclosureLevel, loaded.DisclosureLevel)
	}

	loaded.LastSeenAt = now.Add(2 * time.Hour)
	loaded.DisclosureLevel = "high"
	loaded.RecentOperations = []string{"open", "update"}

	updated, err := repo.Update(ctx, loaded)
	if err != nil {
		t.Fatalf("update session: %v", err)
	}
	if updated.DisclosureLevel != "high" {
		t.Fatalf("expected updated disclosure, got %q", updated.DisclosureLevel)
	}
}

func TestSearchIndexFilters(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	obsRepo := NewObservationRepository(db)
	index := NewSearchIndex(db)

	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	observation := domain.Observation{
		ID:        "obs-search-1",
		Content:   "Findable content",
		CreatedAt: now,
		UpdatedAt: now,
		Project:   "search",
		TopicKey:  "topic-search",
		Tags:      []string{"tag1", "tag2"},
		Source:    "cli",
		Metadata:  map[string]any{"k": "v"},
	}

	if _, err := obsRepo.Create(ctx, observation); err != nil {
		t.Fatalf("create observation: %v", err)
	}
	if err := index.Index(ctx, observation); err != nil {
		t.Fatalf("index observation: %v", err)
	}

	results, err := index.Search(ctx, "Findable", ports.SearchFilter{Project: "search", Tags: []string{"tag1"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].ObservationID != observation.ID {
		t.Fatalf("unexpected search results: %#v", results)
	}

	if _, err := obsRepo.SoftDelete(ctx, observation.ID, now.Add(time.Hour)); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	results, err = index.Search(ctx, "Findable", ports.SearchFilter{IncludeDeleted: false})
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results after delete, got %d", len(results))
	}

	results, err = index.Search(ctx, "Findable", ports.SearchFilter{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("search include deleted: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with include deleted, got %d", len(results))
	}
}
