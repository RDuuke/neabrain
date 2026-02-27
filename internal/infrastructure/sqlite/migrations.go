package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// Migration represents a schema migration step.
type Migration struct {
	Version   int
	Name      string
	Statement string
}

var baseMigrations = []Migration{
	{
		Version: 1,
		Name:    "create_core_tables",
		Statement: `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS observations (
  id TEXT PRIMARY KEY,
  content TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT,
  project TEXT,
  topic_key TEXT,
  tags TEXT,
  source TEXT,
  metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_observations_project ON observations(project);
CREATE INDEX IF NOT EXISTS idx_observations_topic_key ON observations(topic_key);
CREATE INDEX IF NOT EXISTS idx_observations_deleted_at ON observations(deleted_at);

CREATE TABLE IF NOT EXISTS topics (
  id TEXT PRIMARY KEY,
  topic_key TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT,
  metadata TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_topics_topic_key ON topics(topic_key);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  disclosure_level TEXT NOT NULL,
  recent_operations TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS duplicates (
  id TEXT PRIMARY KEY,
  original_observation_id TEXT NOT NULL,
  duplicate_observation_id TEXT NOT NULL,
  reason TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_duplicates_original ON duplicates(original_observation_id);
CREATE INDEX IF NOT EXISTS idx_duplicates_duplicate ON duplicates(duplicate_observation_id);
`,
	},
	{
		Version: 2,
		Name:    "create_observations_fts",
		Statement: `
CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
  content,
  observation_id UNINDEXED
);

INSERT INTO observations_fts (observation_id, content)
SELECT id, content FROM observations;
`,
	},
}

// ApplyMigrations ensures all base migrations are applied.
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL);"); err != nil {
		return err
	}

	applied, err := loadAppliedVersions(ctx, db)
	if err != nil {
		return err
	}

	sort.Slice(baseMigrations, func(i, j int) bool {
		return baseMigrations[i].Version < baseMigrations[j].Version
	})

	for _, migration := range baseMigrations {
		if applied[migration.Version] {
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, migration.Statement); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, datetime('now'));", migration.Version, migration.Name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d (%s): %w", migration.Version, migration.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func loadAppliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return versions, nil
}
