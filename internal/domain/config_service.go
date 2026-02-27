package domain

import (
	"context"
	"fmt"
)

// ConfigService implements configuration use cases.
type ConfigService struct {
	resolver PathResolver
	loader   ConfigLoader
}

func NewConfigService(resolver PathResolver, loader ConfigLoader) *ConfigService {
	return &ConfigService{resolver: resolver, loader: loader}
}

func (s *ConfigService) ResolvePaths(ctx context.Context, overrides ConfigOverrides) (ResolvedPaths, error) {
	if s == nil || s.resolver == nil {
		return ResolvedPaths{}, fmt.Errorf("config resolver unavailable")
	}
	return s.resolver.Resolve(ctx, overrides)
}

func (s *ConfigService) LoadConfig(ctx context.Context, overrides ConfigOverrides) (Config, error) {
	if s == nil || s.loader == nil {
		return Config{}, fmt.Errorf("config loader unavailable")
	}
	return s.loader.Load(ctx, overrides)
}
