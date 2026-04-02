# VS Code integration

## Quick setup

```bash
neabrain setup vscode
```

Add the output to `.vscode/mcp.json`:

```json
{
  "servers": {
    "neabrain": {
      "type": "stdio",
      "command": "/path/to/neabrain",
      "args": ["mcp"]
    }
  }
}
```

## Available tools

All `nbn_*` tools. See [AGENTS.md](../../AGENTS.md#mcp-tools-reference).
