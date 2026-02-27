package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ports "neabrain/internal/ports/outbound"
)

const (
	envStoragePath    = "NEABRAIN_STORAGE_PATH"
	envFTSPath        = "NEABRAIN_FTS_PATH"
	envDefaultProject = "NEABRAIN_DEFAULT_PROJECT"
	envDedupePolicy   = "NEABRAIN_DEDUPE_POLICY"
	envConfigFile     = "NEABRAIN_CONFIG_FILE"
)

type ConfigAdapter struct{}

var _ ports.PathResolver = (*ConfigAdapter)(nil)
var _ ports.ConfigLoader = (*ConfigAdapter)(nil)

func NewConfigAdapter() *ConfigAdapter {
	return &ConfigAdapter{}
}

func (a *ConfigAdapter) Resolve(ctx context.Context, overrides ports.ConfigOverrides) (ports.ResolvedPaths, error) {
	defaults, err := resolveDefaults()
	if err != nil {
		return ports.ResolvedPaths{}, err
	}

	configPath := resolveConfigFilePath(overrides, defaults)
	fileConfig, err := loadFileConfig(configPath)
	if err != nil {
		return ports.ResolvedPaths{}, err
	}

	storagePath := pickFirst(
		valueFromPtr(overrides.StoragePath),
		strings.TrimSpace(os.Getenv(envStoragePath)),
		fileConfig.StoragePath,
		defaults.StoragePath,
	)
	ftsPath := pickFirst(
		valueFromPtr(overrides.FTSPath),
		strings.TrimSpace(os.Getenv(envFTSPath)),
		fileConfig.FTSPath,
		defaults.FTSPath,
	)
	if strings.TrimSpace(ftsPath) == "" {
		ftsPath = storagePath
	}

	return ports.ResolvedPaths{StoragePath: storagePath, FTSPath: ftsPath}, nil
}

func (a *ConfigAdapter) Load(ctx context.Context, overrides ports.ConfigOverrides) (ports.Config, error) {
	defaults, err := resolveDefaults()
	if err != nil {
		return ports.Config{}, err
	}

	configPath := resolveConfigFilePath(overrides, defaults)
	fileConfig, err := loadFileConfig(configPath)
	if err != nil {
		return ports.Config{}, err
	}

	storagePath := pickFirst(
		valueFromPtr(overrides.StoragePath),
		strings.TrimSpace(os.Getenv(envStoragePath)),
		fileConfig.StoragePath,
		defaults.StoragePath,
	)
	ftsPath := pickFirst(
		valueFromPtr(overrides.FTSPath),
		strings.TrimSpace(os.Getenv(envFTSPath)),
		fileConfig.FTSPath,
		defaults.FTSPath,
	)
	if strings.TrimSpace(ftsPath) == "" {
		ftsPath = storagePath
	}
	defaultProject := pickFirst(
		valueFromPtr(overrides.DefaultProject),
		strings.TrimSpace(os.Getenv(envDefaultProject)),
		fileConfig.DefaultProject,
		defaults.DefaultProject,
	)
	dedupePolicy := pickFirst(
		valueFromPtr(overrides.DedupePolicy),
		strings.TrimSpace(os.Getenv(envDedupePolicy)),
		fileConfig.DedupePolicy,
		defaults.DedupePolicy,
	)
	validatedDedupe, err := normalizeDedupePolicy(dedupePolicy, defaults.DedupePolicy)
	if err != nil {
		return ports.Config{}, err
	}

	return ports.Config{StoragePath: storagePath, FTSPath: ftsPath, DefaultProject: defaultProject, DedupePolicy: validatedDedupe}, nil
}

type fileConfig struct {
	StoragePath    string `json:"storage_path"`
	FTSPath        string `json:"fts_path"`
	DefaultProject string `json:"default_project"`
	DedupePolicy   string `json:"dedupe_policy"`
}

type configDefaults struct {
	StoragePath    string
	FTSPath        string
	DefaultProject string
	DedupePolicy   string
	ConfigFile     string
}

func resolveDefaults() (configDefaults, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return configDefaults{}, err
	}

	baseDir := filepath.Join(configDir, "neabrain")
	storagePath := filepath.Join(baseDir, "neabrain.db")
	return configDefaults{
		StoragePath:    storagePath,
		FTSPath:        storagePath,
		DefaultProject: "",
		DedupePolicy:   "exact",
		ConfigFile:     filepath.Join(baseDir, "config.json"),
	}, nil
}

func resolveConfigFilePath(overrides ports.ConfigOverrides, defaults configDefaults) string {
	return pickFirst(
		valueFromPtr(overrides.ConfigFile),
		strings.TrimSpace(os.Getenv(envConfigFile)),
		defaults.ConfigFile,
	)
}

func loadFileConfig(path string) (fileConfig, error) {
	if strings.TrimSpace(path) == "" {
		return fileConfig{}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fileConfig{}, nil
		}
		return fileConfig{}, err
	}
	if info.IsDir() {
		return fileConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, err
	}

	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fileConfig{}, err
	}

	baseDir := filepath.Dir(path)
	cfg.StoragePath = resolveRelativePath(baseDir, cfg.StoragePath)
	cfg.FTSPath = resolveRelativePath(baseDir, cfg.FTSPath)

	return cfg, nil
}

func resolveRelativePath(baseDir string, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(baseDir, trimmed)
}

func pickFirst(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func valueFromPtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func normalizeDedupePolicy(value string, fallback string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return strings.ToLower(strings.TrimSpace(fallback)), nil
	}
	if trimmed == "exact" || trimmed == "none" {
		return trimmed, nil
	}
	return "", fmt.Errorf("unknown dedupe policy: %s", value)
}
