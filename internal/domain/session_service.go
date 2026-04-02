package domain

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// SessionService implements session lifecycle use cases.
type SessionService struct {
	repo        SessionRepository
	clock       Clock
	idGenerator func() string
}

func NewSessionService(repo SessionRepository, clock Clock) *SessionService {
	return &SessionService{
		repo:        repo,
		clock:       clock,
		idGenerator: defaultSessionID,
	}
}

func (s *SessionService) Open(ctx context.Context, input SessionOpenInput) (Session, error) {
	if strings.TrimSpace(input.DisclosureLevel) == "" {
		return Session{}, NewInvalidInput("disclosure level is required")
	}

	now := s.clock.Now()
	session := Session{
		ID:              s.idGenerator(),
		CreatedAt:       now,
		LastSeenAt:      now,
		DisclosureLevel: input.DisclosureLevel,
	}

	return s.repo.Create(ctx, session)
}

func (s *SessionService) Read(ctx context.Context, id string) (Session, error) {
	if strings.TrimSpace(id) == "" {
		return Session{}, NewInvalidInput("session id is required")
	}
	return s.repo.GetByID(ctx, id)
}

func (s *SessionService) Resume(ctx context.Context, id string) (Session, error) {
	if strings.TrimSpace(id) == "" {
		return Session{}, NewInvalidInput("session id is required")
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Session{}, err
	}

	existing.LastSeenAt = s.clock.Now()
	return s.repo.Update(ctx, existing)
}

func (s *SessionService) UpdateDisclosure(ctx context.Context, id string, disclosureLevel string) (Session, error) {
	if strings.TrimSpace(id) == "" {
		return Session{}, NewInvalidInput("session id is required")
	}
	if strings.TrimSpace(disclosureLevel) == "" {
		return Session{}, NewInvalidInput("disclosure level is required")
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Session{}, err
	}

	existing.DisclosureLevel = disclosureLevel
	existing.LastSeenAt = s.clock.Now()

	return s.repo.Update(ctx, existing)
}

func defaultSessionID() string {
	return uuid.NewString()
}
