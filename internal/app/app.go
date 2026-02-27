package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"neabrain/internal/adapters/outbound"
	"neabrain/internal/adapters/outbound/sqlite"
	"neabrain/internal/domain"
	infra "neabrain/internal/infrastructure/sqlite"
	"neabrain/internal/observability"
	ports "neabrain/internal/ports/outbound"
)

// App bundles configured services and resources.
type App struct {
	Config             ports.Config
	ConfigService      *domain.ConfigService
	ObservationService *domain.ObservationService
	SearchService      *domain.SearchService
	TopicService       *domain.TopicService
	SessionService     *domain.SessionService
	Logger             *observability.Logger
	Metrics            *observability.Metrics
	db                 *sql.DB
}

// Bootstrap wires adapters, repositories, and services.
func Bootstrap(ctx context.Context, overrides ports.ConfigOverrides) (*App, error) {
	logger := observability.DefaultLogger()
	metrics := observability.DefaultMetrics()

	configAdapter := outbound.NewConfigAdapter()
	configService := domain.NewConfigService(configAdapter, configAdapter)
	logger.Info("config load", map[string]any{"adapter": "config"})
	metrics.Inc("adapter.config.load")
	cfg, err := configService.LoadConfig(ctx, overrides)
	if err != nil {
		logger.Error("config load failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	dedupePolicy, err := resolveDedupePolicy(cfg.DedupePolicy)
	if err != nil {
		return nil, err
	}

	db, err := infra.Open(ctx, infra.Config{Path: cfg.StoragePath, MaxOpenConns: 1})
	if err != nil {
		return nil, err
	}

	if err := infra.ApplyMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	observationRepo := outbound.WrapObservationRepository(sqlite.NewObservationRepository(db), logger, metrics)
	topicRepo := outbound.WrapTopicRepository(sqlite.NewTopicRepository(db), logger, metrics)
	sessionRepo := outbound.WrapSessionRepository(sqlite.NewSessionRepository(db), logger, metrics)
	duplicateRepo := outbound.WrapDuplicateRepository(sqlite.NewDuplicateRepository(db), logger, metrics)
	searchIndex := outbound.WrapSearchIndex(sqlite.NewSearchIndex(db), logger, metrics)
	clock := outbound.NewClockAdapter()

	app := &App{
		Config:             cfg,
		ConfigService:      configService,
		ObservationService: domain.NewObservationService(observationRepo, searchIndex, clock, duplicateRepo, dedupePolicy),
		SearchService:      domain.NewSearchService(observationRepo, searchIndex),
		TopicService:       domain.NewTopicService(topicRepo, clock),
		SessionService:     domain.NewSessionService(sessionRepo, clock),
		Logger:             logger,
		Metrics:            metrics,
		db:                 db,
	}

	return app, nil
}

// Close releases database resources.
func (a *App) Close() error {
	if a == nil || a.db == nil {
		return nil
	}
	return a.db.Close()
}

func resolveDedupePolicy(value string) (domain.DedupePolicy, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" || trimmed == domain.DedupePolicyExact {
		return domain.ExactMatchDedupePolicy{}, nil
	}
	if trimmed == domain.DedupePolicyNone {
		return domain.NoopDedupePolicy{}, nil
	}
	return nil, fmt.Errorf("unknown dedupe policy: %s", value)
}
