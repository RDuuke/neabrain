package domain

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// TopicService implements topic management use cases.
type TopicService struct {
	repo        TopicRepository
	clock       Clock
	idGenerator func() string
}

func NewTopicService(repo TopicRepository, clock Clock) *TopicService {
	return &TopicService{
		repo:        repo,
		clock:       clock,
		idGenerator: defaultTopicID,
	}
}

func (s *TopicService) UpsertByTopicKey(ctx context.Context, input TopicUpsertInput) (Topic, error) {
	if strings.TrimSpace(input.TopicKey) == "" {
		return Topic{}, NewInvalidInput("topic key is required")
	}

	now := s.clock.Now()
	topic := Topic{
		ID:          s.idGenerator(),
		TopicKey:    input.TopicKey,
		Name:        input.Name,
		Description: input.Description,
		Metadata:    input.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return s.repo.UpsertByTopicKey(ctx, topic)
}

func (s *TopicService) List(ctx context.Context) ([]TopicSummary, error) {
	return s.repo.List(ctx)
}

func defaultTopicID() string {
	return uuid.NewString()
}
