package domain

import "time"

// Observation represents a captured memory item.
type Observation struct {
	ID        string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	Project  string
	TopicKey string
	Tags     []string
	Source   string
	Metadata map[string]any
}

// Topic represents a named grouping for observations.
type Topic struct {
	ID          string
	TopicKey    string
	Name        string
	Description string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Session represents an interactive user session.
type Session struct {
	ID               string
	CreatedAt        time.Time
	LastSeenAt       time.Time
	DisclosureLevel  string
	RecentOperations []string
}

// Duplicate captures a dedupe relationship between observations.
type Duplicate struct {
	ID                     string
	OriginalObservationID  string
	DuplicateObservationID string
	Reason                 string
	CreatedAt              time.Time
}

// ProjectSummary summarizes observation counts per project.
type ProjectSummary struct {
	Name  string
	Count int
}

// ObservationStats holds aggregate counts about the observation store.
type ObservationStats struct {
	Active   int
	Deleted  int
	Projects int
}
