# Claude Code integration

NeaBrain exposes all its tools via MCP. Claude Code connects to it as an MCP server.

## Quick setup

Run this command to print the config snippet:

```bash
neabrain setup claude-code
```

Then add the printed JSON to `.claude/settings.json` under the `mcpServers` key:

```json
{
  "mcpServers": {
    "neabrain": {
      "type": "stdio",
      "command": "/path/to/neabrain",
      "args": ["mcp"]
    }
  }
}
```

Replace `/path/to/neabrain` with the actual path (or just `neabrain` if it is in `PATH`).

## Available tools

All `nbn_*` tools are available. See [AGENTS.md](../../AGENTS.md#mcp-tools-reference) for the full list.

## Configuration

Set environment variables before starting Claude Code:

| Variable | Default | Description |
|---|---|---|
| `NEABRAIN_DEFAULT_PROJECT` | `""` | Default project for new observations |
| `NEABRAIN_STORAGE_PATH` | `~/.config/neabrain/neabrain.db` | Database path |
| `NEABRAIN_DEDUPE_POLICY` | `exact` | Deduplication policy (`exact` or `none`) |
