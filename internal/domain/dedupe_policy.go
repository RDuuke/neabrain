package domain

import (
	"context"
	"strings"
)

const (
	DedupeReasonExactContent = "exact_content_match"
	DedupePolicyExact        = "exact"
	DedupePolicyNone         = "none"
)

// DedupeMatch captures a duplicate relationship with reason.
type DedupeMatch struct {
	OriginalObservationID string
	Reason                string
}

// DedupePolicy evaluates potential duplicates for an observation.
type DedupePolicy interface {
	FindDuplicates(ctx context.Context, candidate Observation, repo ObservationRepository) ([]DedupeMatch, error)
}

// ExactMatchDedupePolicy implements strict content matching within a project.
type ExactMatchDedupePolicy struct{}

func (p ExactMatchDedupePolicy) FindDuplicates(ctx context.Context, candidate Observation, repo ObservationRepository) ([]DedupeMatch, error) {
	content := strings.TrimSpace(candidate.Content)
	if content == "" {
		return nil, NewInvalidInput("observation content is required")
	}

	observations, err := repo.FindByContent(ctx, candidate.Content, candidate.Project, false)
	if err != nil {
		return nil, err
	}

	matches := make([]DedupeMatch, 0, len(observations))
	for _, observation := range observations {
		if observation.ID == candidate.ID {
			continue
		}
		matches = append(matches, DedupeMatch{
			OriginalObservationID: observation.ID,
			Reason:                DedupeReasonExactContent,
		})
	}

	return matches, nil
}

// NoopDedupePolicy disables duplicate detection.
type NoopDedupePolicy struct{}

func (p NoopDedupePolicy) FindDuplicates(ctx context.Context, candidate Observation, repo ObservationRepository) ([]DedupeMatch, error) {
	return nil, nil
}
