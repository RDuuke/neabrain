package domain

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ObservationService implements observation use cases.
type ObservationService struct {
	repo        ObservationRepository
	searchIndex SearchIndex
	duplicate   DuplicateRepository
	clock       Clock
	dedupe      DedupePolicy
	idGenerator func() string
	duplicateID func() string
}

func NewObservationService(
	repo ObservationRepository,
	searchIndex SearchIndex,
	clock Clock,
	duplicateRepo DuplicateRepository,
	dedupePolicy DedupePolicy,
) *ObservationService {
	if dedupePolicy == nil {
		dedupePolicy = ExactMatchDedupePolicy{}
	}
	return &ObservationService{
		repo:        repo,
		searchIndex: searchIndex,
		duplicate:   duplicateRepo,
		clock:       clock,
		dedupe:      dedupePolicy,
		idGenerator: defaultObservationID,
		duplicateID: defaultDuplicateID,
	}
}

func (s *ObservationService) Create(ctx context.Context, input ObservationCreateInput) (Observation, error) {
	trimmed := strings.TrimSpace(input.Content)
	if trimmed == "" {
		return Observation{}, NewInvalidInput("observation content is required")
	}

	now := s.clock.Now()
	observation := Observation{
		ID:        s.idGenerator(),
		Content:   input.Content,
		CreatedAt: now,
		UpdatedAt: now,
		Project:   input.Project,
		TopicKey:  input.TopicKey,
		Tags:      input.Tags,
		Source:    input.Source,
		Metadata:  input.Metadata,
	}

	duplicates, err := s.findDuplicates(ctx, observation)
	if err != nil {
		return Observation{}, err
	}
	if len(duplicates) > 0 && !input.AllowDuplicate {
		return Observation{}, NewConflict("duplicate observation detected: " + duplicates[0].OriginalObservationID)
	}

	created, err := s.repo.Create(ctx, observation)
	if err != nil {
		return Observation{}, err
	}

	if len(duplicates) > 0 {
		if s.duplicate == nil {
			return Observation{}, NewConflict("duplicate tracking unavailable")
		}
		if err := s.recordDuplicates(ctx, created, duplicates); err != nil {
			return Observation{}, err
		}
	}

	if err := s.searchIndex.Index(ctx, created); err != nil {
		if compErr := s.compensateIndexFailureOnCreate(ctx, created); compErr != nil {
			return Observation{}, fmt.Errorf("index observation: %w; compensation failed: %v", err, compErr)
		}
		return Observation{}, err
	}

	return created, nil
}

func (s *ObservationService) Read(ctx context.Context, id string, includeDeleted bool) (Observation, error) {
	if strings.TrimSpace(id) == "" {
		return Observation{}, NewInvalidInput("observation id is required")
	}
	return s.repo.GetByID(ctx, id, includeDeleted)
}

func (s *ObservationService) Update(ctx context.Context, input ObservationUpdateInput) (Observation, error) {
	if strings.TrimSpace(input.ID) == "" {
		return Observation{}, NewInvalidInput("observation id is required")
	}

	existing, err := s.repo.GetByID(ctx, input.ID, false)
	if err != nil {
		return Observation{}, err
	}
	original := existing

	if input.Content != nil {
		trimmed := strings.TrimSpace(*input.Content)
		if trimmed == "" {
			return Observation{}, NewInvalidInput("observation content is required")
		}
		existing.Content = *input.Content
	}
	if input.Project != nil {
		existing.Project = *input.Project
	}
	if input.TopicKey != nil {
		existing.TopicKey = *input.TopicKey
	}
	if input.Tags != nil {
		existing.Tags = input.Tags
	}
	if input.Source != nil {
		existing.Source = *input.Source
	}
	if input.Metadata != nil {
		existing.Metadata = input.Metadata
	}

	if shouldCheckDedupe(input) {
		duplicates, err := s.findDuplicates(ctx, existing)
		if err != nil {
			return Observation{}, err
		}
		if len(duplicates) > 0 {
			return Observation{}, NewConflict("duplicate observation detected: " + duplicates[0].OriginalObservationID)
		}
	}

	existing.UpdatedAt = s.clock.Now()
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return Observation{}, err
	}

	if err := s.searchIndex.Index(ctx, updated); err != nil {
		if compErr := s.compensateIndexFailureOnUpdate(ctx, original); compErr != nil {
			return Observation{}, fmt.Errorf("index observation: %w; compensation failed: %v", err, compErr)
		}
		return Observation{}, err
	}

	return updated, nil
}

func (s *ObservationService) List(ctx context.Context, filter ObservationListFilter) ([]Observation, error) {
	return s.repo.List(ctx, filter)
}

func (s *ObservationService) GetStats(ctx context.Context) (ObservationStats, error) {
	return s.repo.GetStats(ctx)
}

func (s *ObservationService) ListProjects(ctx context.Context) ([]ProjectSummary, error) {
	return s.repo.ListProjects(ctx)
}

func (s *ObservationService) MergeProjects(ctx context.Context, from []string, to string) (int, error) {
	to = strings.TrimSpace(to)
	if to == "" {
		return 0, NewInvalidInput("target project name is required")
	}
	total := 0
	for _, src := range from {
		src = strings.TrimSpace(src)
		if src == "" || src == to {
			continue
		}
		n, err := s.repo.RenameProject(ctx, src, to)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (s *ObservationService) RenameProject(ctx context.Context, oldName, newName string) (int, error) {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" {
		return 0, NewInvalidInput("old project name is required")
	}
	if newName == "" {
		return 0, NewInvalidInput("new project name is required")
	}
	return s.repo.RenameProject(ctx, oldName, newName)
}

func (s *ObservationService) SoftDelete(ctx context.Context, id string) (Observation, error) {
	if strings.TrimSpace(id) == "" {
		return Observation{}, NewInvalidInput("observation id is required")
	}
	deletedAt := s.clock.Now()
	return s.repo.SoftDelete(ctx, id, deletedAt)
}

func defaultObservationID() string {
	return uuid.NewString()
}

func defaultDuplicateID() string {
	return uuid.NewString()
}

func (s *ObservationService) findDuplicates(ctx context.Context, candidate Observation) ([]DedupeMatch, error) {
	if s.dedupe == nil {
		return nil, nil
	}
	return s.dedupe.FindDuplicates(ctx, candidate, s.repo)
}

func (s *ObservationService) recordDuplicates(ctx context.Context, created Observation, matches []DedupeMatch) error {
	for _, match := range matches {
		duplicate := Duplicate{
			ID:                     s.duplicateID(),
			OriginalObservationID:  match.OriginalObservationID,
			DuplicateObservationID: created.ID,
			Reason:                 match.Reason,
			CreatedAt:              s.clock.Now(),
		}
		if _, err := s.duplicate.Create(ctx, duplicate); err != nil {
			return err
		}
	}
	return nil
}

func shouldCheckDedupe(input ObservationUpdateInput) bool {
	return input.Content != nil || input.Project != nil
}

func (s *ObservationService) compensateIndexFailureOnCreate(ctx context.Context, created Observation) error {
	if s == nil || s.repo == nil || s.clock == nil {
		return fmt.Errorf("compensation unavailable")
	}
	_, err := s.repo.SoftDelete(ctx, created.ID, s.clock.Now())
	return err
}

func (s *ObservationService) compensateIndexFailureOnUpdate(ctx context.Context, previous Observation) error {
	if s == nil || s.repo == nil || s.searchIndex == nil {
		return fmt.Errorf("compensation unavailable")
	}
	restored, err := s.repo.Update(ctx, previous)
	if err != nil {
		return err
	}
	return s.searchIndex.Index(ctx, restored)
}
