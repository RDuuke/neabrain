package domain

import (
	"context"
	"errors"
	"strings"
)

// SearchService implements full-text search use cases.
type SearchService struct {
	repo        ObservationRepository
	searchIndex SearchIndex
}

func NewSearchService(repo ObservationRepository, searchIndex SearchIndex) *SearchService {
	return &SearchService{repo: repo, searchIndex: searchIndex}
}

func (s *SearchService) Search(ctx context.Context, query string, filter SearchFilter) ([]Observation, error) {
	if strings.TrimSpace(query) == "" {
		return nil, NewInvalidInput("search query is required")
	}

	results, err := s.searchIndex.Search(ctx, query, filter)
	if err != nil {
		return nil, err
	}

	observations := make([]Observation, 0, len(results))
	for _, result := range results {
		observation, err := s.repo.GetByID(ctx, result.ObservationID, filter.IncludeDeleted)
		if err != nil {
			var domainErr DomainError
			if errors.As(err, &domainErr) && domainErr.Code == ErrorNotFound {
				_ = s.searchIndex.Remove(ctx, result.ObservationID)
				continue
			}
			return nil, err
		}
		if !isObservationVisible(observation.Tags, filter.DisclosureLevel) {
			continue
		}
		observations = append(observations, observation)
	}

	return observations, nil
}
