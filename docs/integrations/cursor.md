# Cursor integration

## Quick setup

```bash
neabrain setup cursor
```

Add the output to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "neabrain": {
      "command": "/path/to/neabrain",
      "args": ["mcp"]
    }
  }
}
```

## Available tools

All `nbn_*` tools. See [AGENTS.md](../../AGENTS.md#mcp-tools-reference).
