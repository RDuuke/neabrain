package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallAndUninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows

	agents := []Agent{AgentClaudeCode, AgentCursor, AgentVSCode, AgentOpenCode}

	for _, agent := range agents {
		t.Run(string(agent), func(t *testing.T) {
			// First install
			res, err := Install(agent, "/usr/local/bin/neabrain")
			if err != nil {
				t.Fatalf("Install failed: %v", err)
			}
			if res.AlreadySet {
				t.Fatal("expected fresh install, got AlreadySet")
			}

			// Config file must exist
			if _, err := os.Stat(res.ConfigFile); err != nil {
				t.Fatalf("config file missing: %v", err)
			}

			// Idempotency: second install returns AlreadySet
			res2, err := Install(agent, "/usr/local/bin/neabrain")
			if err != nil {
				t.Fatalf("second Install failed: %v", err)
			}
			if !res2.AlreadySet {
				t.Fatal("expected AlreadySet on second install")
			}

			// Uninstall removes the entry
			res3, err := Uninstall(agent)
			if err != nil {
				t.Fatalf("Uninstall failed: %v", err)
			}
			if !res3.Removed {
				t.Fatal("expected Removed = true")
			}

			// Second uninstall is a no-op
			res4, err := Uninstall(agent)
			if err != nil {
				t.Fatalf("second Uninstall failed: %v", err)
			}
			if res4.Removed {
				t.Fatal("expected Removed = false on second uninstall")
			}
		})
	}
}

func TestInstallMergesExistingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Pre-create a config with an unrelated key
	cfgPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := map[string]any{"theme": "dark"}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Install(AgentClaudeCode, "/usr/local/bin/neabrain")
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Existing key must be preserved
	raw, _ := os.ReadFile(cfgPath)
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg["theme"] != "dark" {
		t.Fatalf("existing key lost: %v", cfg)
	}
	if cfg["mcpServers"] == nil {
		t.Fatal("mcpServers missing after install")
	}
}
