# OpenCode MCP Plugin (NeaBrain)

This repo includes a packaged OpenCode MCP plugin that launches NeaBrain's MCP server and
loads a minimal OpenCode adapter for memory-aware behavior.

## Plugin Package
- Location: `plugins/opencode-mcp`
- Manifest: `plugins/opencode-mcp/opencode.plugin.json`
- Purpose: run `neabrain mcp` as a local MCP server and load a tool-mapping adapter

## Install From Repo
Prereqs: Go 1.22+ and a built `neabrain` binary.

1) Build NeaBrain (from repo root):
```bash
go build -o neabrain ./cmd/neabrain
```

2) Copy the plugin package into your OpenCode plugins directory:
```bash
# macOS / Linux
mkdir -p ~/.opencode/plugins
cp -R ./plugins/opencode-mcp ~/.opencode/plugins/neabrain-mcp

# Windows (PowerShell)
mkdir $env:USERPROFILE\.opencode\plugins -Force
Copy-Item -Recurse .\plugins\opencode-mcp $env:USERPROFILE\.opencode\plugins\neabrain-mcp
```

3) Register the plugin in `~/.opencode/opencode.json`:
```json
{
  "plugins": [
    { "path": "~/.opencode/plugins/neabrain-mcp" }
  ]
}
```

4) Ensure `@opencode-ai/plugin` is available (OpenCode loads dependencies from
`~/.config/opencode/package.json` or the project `.opencode/package.json`).

Example package.json:
```json
{
  "dependencies": {
    "@opencode-ai/plugin": "1.2.6"
  }
}
```

## Usage
1) Ensure `neabrain` is on your PATH, or update the plugin manifest command to a full path.
2) Start OpenCode. The plugin will expose NeaBrain MCP tools plus `nbn_` aliases such as:
    - observation.create
    - observation.read
    - observation.update
    - observation.list
    - observation.delete
    - search
    - topic.upsert
    - session.open
    - session.resume
    - session.update_disclosure
    - config.show
    - nbn_observation_create
    - nbn_search
    - nbn_session_summary
    - nbn_context

### Adapter behavior
- Adds a short system instruction guiding memory usage.
- Stores a lightweight session summary on compaction.
- Pulls NeaBrain context before and after compaction.

### Timeouts, retries, and diagnostics
The adapter enforces read deadlines and retries for read-only tools via environment variables in the
plugin manifest:

- `NEABRAIN_MCP_READ_TIMEOUT_MS` (default: 15000): per-call deadline for read-only tools.
- `NEABRAIN_MCP_READ_RETRIES` (default: 2): retry count for read-only tools.
- `NEABRAIN_MCP_READ_RETRY_BACKOFF_MS` (default: 200): backoff between retries.
- `NEABRAIN_MCP_DIAGNOSTICS` (default: "0"): set to "1" for duration + error category logs.

Retries apply only to read-only tools: `nbn_observation_read`, `nbn_observation_list`, `nbn_search`,
`nbn_config_show`, and `nbn_context`. Each read tool call includes a `deadline_ms` parameter so the
MCP server can honor the timeout.

### Optional: Custom Config
You can set config in the plugin manifest with environment variables if needed:
```json
{
  "mcp": {
    "command": "neabrain",
    "args": ["mcp"],
    "env": {
      "NEABRAIN_CONFIG_FILE": "~/path/to/config.json"
    }
  }
}
```

## Troubleshooting
- If OpenCode cannot find the binary, set a full path in `opencode.plugin.json`.
- If storage paths are wrong, set `NEABRAIN_STORAGE_PATH` or `NEABRAIN_CONFIG_FILE` in the plugin manifest.
