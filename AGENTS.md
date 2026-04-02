# AGENTS.md — Skills Registry for NeaBrain

This file tells AI agents (Claude Code, OpenCode, Cursor, etc.) how to orient themselves in this codebase and which patterns to follow for each type of task.

## How to use this file

1. Identify which area your task touches (see table below).
2. Read the corresponding key files before making changes.
3. Follow the conventions listed for that area.
4. Run the verification command for that area before committing.

---

## Architecture overview

NeaBrain is a **single-user memory system** built with **hexagonal architecture** (ports and adapters):

```
cmd/neabrain/          ← CLI entry point
internal/
  domain/              ← Core logic + entities. NO external imports except uuid.
  ports/
    inbound/           ← Service interfaces consumed by adapters
    outbound/          ← Repository + infrastructure interfaces
  adapters/
    inbound/           ← CLI, HTTP, MCP, TUI  (implement inbound ports)
    outbound/          ← SQLite, config, clock (implement outbound ports)
  app/                 ← Dependency injection (Bootstrap)
  infrastructure/      ← DB open + migrations
  observability/       ← Logger + metrics
plugins/opencode-mcp/  ← TypeScript adapter for OpenCode
docs/                  ← User-facing documentation
```

**Dependency rule:** domain → nothing external. Adapters depend on domain, never the reverse.

---

## Skill registry

| Task type | Key files to read | Conventions | Verify with |
|---|---|---|---|
| **Domain logic** | `internal/domain/*.go` | No external imports; use `domain.NewInvalidInput/NewNotFound/NewConflict`; add method to service + port interface | `go test ./internal/domain/...` |
| **Repository (SQLite)** | `internal/adapters/outbound/sqlite/observation_repository.go`, `internal/domain/outbound_ports.go` | Raw SQL, no ORM; add method to `ObservationRepository` interface first, then SQLite impl, then observability wrapper | `go test ./internal/adapters/outbound/...` |
| **CLI command** | `internal/adapters/inbound/cli/cli.go` | Use `flag.NewFlagSet`; call `withApp()`; write JSON output with `writeJSON()`; add usage string; update `writeUsage()` | `go build ./cmd/neabrain` |
| **MCP tool** | `internal/adapters/inbound/mcp/server.go` | Add args struct → case in `handleToolsCall()` → entry in `toolDefinitions()`; alias pattern `nbn_*` → canonical | `go test ./internal/adapters/inbound/mcp/...` |
| **HTTP endpoint** | `internal/adapters/inbound/http/server.go` | Standard `net/http`; decode JSON body; use domain service; return appropriate status codes | `go test ./internal/adapters/inbound/http/...` |
| **OpenCode plugin** | `plugins/opencode-mcp/adapter.ts` | TypeScript; spawn `neabrain mcp` subprocess; use `callMcpToolWithRetry()` for read-only tools; read-only tools get deadline | Manual test with OpenCode |
| **Configuration** | `internal/domain/outbound_ports.go` (Config), `internal/adapters/outbound/config_adapter.go` | Precedence: CLI > env > config file > default; document new env vars in `docs/operations.md` | `go build ./...` |
| **Testing** | `internal/domain/*_test.go`, `internal/adapters/outbound/sqlite/adapter_integration_test.go` | Integration tests hit real SQLite (no mocks for DB); domain tests use in-memory fakes | `go test ./...` |
| **TUI** | `internal/adapters/inbound/tui/` | Bubble Tea framework; keep view pure, update in message handler | Manual or `go test ./internal/adapters/inbound/tui/...` |
| **Docs** | `docs/`, `README.md`, `AGENTS.md` | Spanish README; English docs in `docs/`; keep `docs/operations.md` up to date for new config | Read docs in context |

---

## Key conventions

### Adding a new feature end-to-end

Follow this order to avoid compile errors:

1. **Entity** (if new struct needed): `internal/domain/entities.go`
2. **Repository interface** (if new DB method): `internal/domain/outbound_ports.go`
3. **SQLite implementation**: `internal/adapters/outbound/sqlite/<entity>_repository.go`
4. **Observability wrapper**: `internal/adapters/outbound/observability.go` (mirror each new method)
5. **Service method**: `internal/domain/<entity>_service.go`
6. **CLI command**: `internal/adapters/inbound/cli/cli.go`
7. **MCP tool**: `internal/adapters/inbound/mcp/server.go`
8. **HTTP endpoint** (optional): `internal/adapters/inbound/http/server.go`
9. **Tests**: update in-memory fakes in `*_test.go` to satisfy interface

### Error handling

- Use `domain.NewInvalidInput(msg)`, `domain.NewNotFound(msg)`, `domain.NewConflict(msg)` in domain.
- In MCP, errors flow through `rpcErrorFrom(err)` — maps domain errors + context errors to RPC codes.
- In CLI, `handleError` maps domain errors to exit codes (2=invalid, 3=not found, 4=conflict).

### MCP deadline support

The MCP server accepts `deadline_ms` in tool calls. Pass it through `callCtx` (already wired). Read-only tools (`nbn_observation_read`, `nbn_observation_list`, `nbn_search`, `nbn_config_show`, `nbn_context`, `nbn_export`, `nbn_projects_list`) should be included in the retry-eligible set in `plugins/opencode-mcp/adapter.ts`.

### No backwards-compatibility stubs

When renaming or removing a function, delete old code. Don't add `// deprecated` wrappers.

---

## Quick start for a new agent session

```bash
# 1. Check what changed recently
git log --oneline -10

# 2. Build to confirm clean state
go build ./...

# 3. Run tests
go test ./...

# 4. Start hacking
```

---

## MCP tools reference

| Tool | Description | Retry-eligible |
|---|---|---|
| `nbn_observation_create` | Create a memory observation | No |
| `nbn_observation_read` | Read observation by ID | Yes |
| `nbn_observation_update` | Update an observation | No |
| `nbn_observation_list` | List observations with filters | Yes |
| `nbn_observation_delete` | Soft-delete an observation | No |
| `nbn_search` | Full-text search | Yes |
| `nbn_export` | Export observations as JSON list | Yes |
| `nbn_topic_upsert` | Create or update a topic | No |
| `nbn_session_open` | Open a new session | No |
| `nbn_session_resume` | Resume existing session | No |
| `nbn_session_update_disclosure` | Update disclosure level | No |
| `nbn_session_summary` | Store a session summary (OpenCode) | No |
| `nbn_context` | Context-aware search alias | Yes |
| `nbn_config_show` | Show effective configuration | Yes |
| `nbn_projects_list` | List projects with observation counts | Yes |
| `nbn_projects_rename` | Rename a project across all observations | No |

---

## Agent integrations

Generate the config snippet for your agent:

```bash
neabrain setup claude-code   # → .claude/settings.json
neabrain setup cursor        # → .cursor/mcp.json
neabrain setup vscode        # → .vscode/mcp.json
neabrain setup opencode      # → opencode.json
```

Full setup guides: `docs/integrations/`
