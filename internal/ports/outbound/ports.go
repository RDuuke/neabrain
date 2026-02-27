package outbound

import "neabrain/internal/domain"

// ObservationRepository defines persistence operations for observations.
type ObservationRepository = domain.ObservationRepository

// TopicRepository defines persistence operations for topics.
type TopicRepository = domain.TopicRepository

// SessionRepository defines persistence operations for sessions.
type SessionRepository = domain.SessionRepository

// DuplicateRepository defines persistence operations for duplicate tracking.
type DuplicateRepository = domain.DuplicateRepository

// SearchIndex defines full-text search operations.
type SearchIndex = domain.SearchIndex

// Clock supplies timestamps for deterministic operations.
type Clock = domain.Clock

// PathResolver resolves storage paths based on configuration sources.
type PathResolver = domain.PathResolver

// ConfigLoader loads effective configuration with precedence rules.
type ConfigLoader = domain.ConfigLoader

// ObservationFilter constrains observation listings.
type ObservationFilter = domain.ObservationListFilter

// SearchFilter constrains search results.
type SearchFilter = domain.SearchFilter

// SearchResult represents a search match with score metadata.
type SearchResult = domain.SearchResult

// ConfigOverrides provides adapter-level configuration overrides.
type ConfigOverrides = domain.ConfigOverrides

// ResolvedPaths holds resolved storage paths.
type ResolvedPaths = domain.ResolvedPaths

// Config represents the effective configuration used by the app.
type Config = domain.Config
