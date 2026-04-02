// Package setup handles writing MCP server configuration for supported AI agents.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Agent identifies a supported AI coding agent.
type Agent string

const (
	AgentClaudeCode Agent = "claude-code"
	AgentCursor     Agent = "cursor"
	AgentVSCode     Agent = "vscode"
	AgentOpenCode   Agent = "opencode"
)

// AgentNames returns all supported agent names as strings.
func AgentNames() []string {
	return []string{
		string(AgentClaudeCode),
		string(AgentCursor),
		string(AgentVSCode),
		string(AgentOpenCode),
	}
}

// Result describes what Install or Uninstall did.
type Result struct {
	Agent      Agent
	ConfigFile string
	AlreadySet bool // true when Install found the entry already present
	Removed    bool // true when Uninstall actually removed the entry
}

// Install writes the neabrain MCP entry into the agent's config file.
// If the entry is already present the function returns AlreadySet = true without modifying the file.
func Install(agent Agent, exePath string) (Result, error) {
	target, err := configPath(agent)
	if err != nil {
		return Result{}, err
	}

	data, err := readOrEmpty(target)
	if err != nil {
		return Result{}, err
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		cfg = map[string]any{}
	}

	already := hasEntry(cfg, agent)
	if already {
		return Result{Agent: agent, ConfigFile: target, AlreadySet: true}, nil
	}

	setEntry(cfg, agent, exePath)

	if err := writeJSON(target, cfg); err != nil {
		return Result{}, err
	}
	return Result{Agent: agent, ConfigFile: target}, nil
}

// Uninstall removes the neabrain MCP entry from the agent's config file.
func Uninstall(agent Agent) (Result, error) {
	target, err := configPath(agent)
	if err != nil {
		return Result{}, err
	}

	data, err := readOrEmpty(target)
	if err != nil {
		return Result{}, err
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Result{Agent: agent, ConfigFile: target}, nil // nothing to remove
	}

	removed := removeEntry(cfg, agent)
	if !removed {
		return Result{Agent: agent, ConfigFile: target}, nil
	}

	if err := writeJSON(target, cfg); err != nil {
		return Result{}, err
	}
	return Result{Agent: agent, ConfigFile: target, Removed: true}, nil
}

// configPath returns the absolute path to the agent's MCP config file.
func configPath(agent Agent) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("setup: cannot determine home directory: %w", err)
	}
	switch agent {
	case AgentClaudeCode:
		return filepath.Join(home, ".claude", "settings.json"), nil
	case AgentCursor:
		return filepath.Join(home, ".cursor", "mcp.json"), nil
	case AgentVSCode:
		return vscodeConfigPath(home), nil
	case AgentOpenCode:
		return filepath.Join(home, ".config", "opencode", "config.json"), nil
	default:
		return "", fmt.Errorf("setup: unknown agent %q", agent)
	}
}

func vscodeConfigPath(home string) string {
	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			appdata = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appdata, "Code", "User", "mcp.json")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json")
	default:
		return filepath.Join(home, ".config", "Code", "User", "mcp.json")
	}
}

// hasEntry checks whether the neabrain entry exists in cfg for the given agent.
func hasEntry(cfg map[string]any, agent Agent) bool {
	switch agent {
	case AgentClaudeCode:
		return nestedExists(cfg, "mcpServers", "neabrain")
	case AgentCursor:
		return nestedExists(cfg, "mcpServers", "neabrain")
	case AgentVSCode:
		return nestedExists(cfg, "servers", "neabrain")
	case AgentOpenCode:
		return nestedExists(cfg, "mcp", "servers", "neabrain")
	}
	return false
}

// setEntry writes the neabrain MCP entry into cfg.
func setEntry(cfg map[string]any, agent Agent, exe string) {
	switch agent {
	case AgentClaudeCode:
		servers := getOrCreate(cfg, "mcpServers")
		servers["neabrain"] = map[string]any{
			"type":    "stdio",
			"command": exe,
			"args":    []string{"mcp"},
		}
	case AgentCursor:
		servers := getOrCreate(cfg, "mcpServers")
		servers["neabrain"] = map[string]any{
			"command": exe,
			"args":    []string{"mcp"},
		}
	case AgentVSCode:
		servers := getOrCreate(cfg, "servers")
		servers["neabrain"] = map[string]any{
			"type":    "stdio",
			"command": exe,
			"args":    []string{"mcp"},
		}
	case AgentOpenCode:
		mcp := getOrCreateNested(cfg, "mcp")
		servers := getOrCreate(mcp, "servers")
		servers["neabrain"] = map[string]any{
			"type":    "local",
			"command": []string{exe, "mcp"},
		}
	}
}

// removeEntry deletes the neabrain key from cfg and returns true if it was present.
func removeEntry(cfg map[string]any, agent Agent) bool {
	switch agent {
	case AgentClaudeCode, AgentCursor:
		return deleteNested(cfg, "mcpServers", "neabrain")
	case AgentVSCode:
		return deleteNested(cfg, "servers", "neabrain")
	case AgentOpenCode:
		if mcp, ok := cfg["mcp"].(map[string]any); ok {
			return deleteNested(mcp, "servers", "neabrain")
		}
	}
	return false
}

// --- helpers ---

func nestedExists(m map[string]any, keys ...string) bool {
	cur := m
	for i, k := range keys {
		v, ok := cur[k]
		if !ok {
			return false
		}
		if i == len(keys)-1 {
			return true
		}
		next, ok := v.(map[string]any)
		if !ok {
			return false
		}
		cur = next
	}
	return false
}

func getOrCreate(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	v := map[string]any{}
	m[key] = v
	return v
}

func getOrCreateNested(m map[string]any, key string) map[string]any {
	return getOrCreate(m, key)
}

func deleteNested(m map[string]any, parent, child string) bool {
	p, ok := m[parent].(map[string]any)
	if !ok {
		return false
	}
	if _, exists := p[child]; !exists {
		return false
	}
	delete(p, child)
	return true
}

func readOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []byte("{}"), nil
	}
	return data, err
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("setup: mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("setup: marshal: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
