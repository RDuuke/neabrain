# OpenCode MCP Plugin: NeaBrain

This package lets OpenCode start the NeaBrain MCP server via `neabrain mcp` and
loads a lightweight OpenCode plugin adapter.

## Install
Copy this folder into your OpenCode plugins directory and register it in `~/.opencode/opencode.json`.
Ensure `@opencode-ai/plugin` is installed (via `~/.config/opencode/package.json` or a project
`.opencode/package.json`).

See `docs/opencode-mcp.md` for full install and usage instructions.

## Adapter behavior
- Registers NeaBrain MCP tool aliases prefixed with `nbn_`.
- Injects brief memory instructions into the system prompt.
- On compaction, stores a short session summary and refreshes context.

## Configuration
Timeouts and retries are configurable via plugin env vars:
- `NEABRAIN_MCP_READ_TIMEOUT_MS` (default: 15000)
- `NEABRAIN_MCP_READ_RETRIES` (default: 2)
- `NEABRAIN_MCP_READ_RETRY_BACKOFF_MS` (default: 200)
- `NEABRAIN_MCP_DIAGNOSTICS` (default: "0")

Retries apply only to read-only tools: `nbn_observation_read`, `nbn_observation_list`, `nbn_timeline`,
`nbn_search`, `nbn_config_show`, `nbn_context`, `nbn_sync_status`, and `nbn_topics_list`. Each read
call includes a `deadline_ms` parameter so the MCP server can honor the timeout. Set
`NEABRAIN_MCP_DIAGNOSTICS=1` to log duration and error category for read calls.

See `docs/opencode-mcp.md` for details.
