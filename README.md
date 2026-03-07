# NeaBrain

![NeaBrain Logo](logo.png)

NeaBrain es un sistema de memory single-user que entrega comportamiento consistente en interfaces CLI, HTTP, MCP y TUI. La arquitectura aisla el core domain logic de adapters y storage para que todas las interfaces compartan la misma semantica.

## Metas
- Proveer CRUD, search, dedupe, topic upsert y session behavior consistente entre adapters.
- Mantener las domain rules estables mientras evolucionan storage, FTS e interfaces.
- Soportar operacion local-first con configuration clara y precedence de overrides.

## Arquitectura
NeaBrain sigue una arquitectura hexagonal (ports and adapters):
- Core entities: Observation, Topic, Session, Duplicate.
- Inbound ports: ObservationService, SearchService, TopicService, SessionService, ConfigService.
- Outbound ports: ObservationRepository, TopicRepository, SessionRepository, DuplicateRepository, SearchIndex, Clock.
- Adapters: CLI, HTTP, MCP, TUI; mas implementaciones locales de storage, FTS, config y clock.

Diagrams (Mermaid specs):
- `docs/diagrams/hexagonal-architecture.md`
- `docs/diagrams/data-flow.md`
- `docs/diagrams/storage-schema.md`

## Instalacion

### From source
Prereqs: Go 1.22 o superior.

```bash
git clone <repo-url>
cd MotorBD
go build -o neabrain ./cmd/neabrain
```

Run directly:

```bash
go run ./cmd/neabrain --help
```

### Binary (optional)
Todavia no hay binaries publicados. Placeholder para futuras releases:

```text
# TODO: add release URLs once published
```

## Inicio rapido

Crear una observation:

```bash
./neabrain observation create --content "hello" --project "demo" --topic "onboarding" --tags "cli"
```

Search de observations:

```bash
./neabrain search --query "hello" --project "demo"
```

Run HTTP server:

```bash
./neabrain serve --addr 127.0.0.1:8080
```

Run MCP server:

```bash
./neabrain mcp
```

Run TUI:

```bash
./neabrain tui
```

## Configuration
Defaults, overrides y environment variables estan documentados en `docs/operations.md`.

Resumen:
- Config directory: os.UserConfigDir()/neabrain
- Config file: <config dir>/config.json
- Storage path: <config dir>/neabrain.db
- FTS path: defaults to storage path cuando no se setea
- Precedence: CLI overrides > environment variables > config file > defaults

Environment variables:
- NEABRAIN_STORAGE_PATH
- NEABRAIN_FTS_PATH
- NEABRAIN_DEFAULT_PROJECT
- NEABRAIN_DEDUPE_POLICY
- NEABRAIN_CONFIG_FILE

## CLI

Top-level commands:
- `observation <create|read|update|list|delete>`
- `search`
- `topic upsert`
- `session <open|resume|update-disclosure>`
- `config show`
- `serve`
- `mcp`
- `tui`

Config override flags (available on most commands):
- `--storage-path`
- `--fts-path`
- `--default-project`
- `--dedupe-policy`
- `--config-file`

Example:

```bash
./neabrain observation list --project "demo" --tags "cli" --storage-path ./data/neabrain.db
```

## HTTP API
Todos los endpoints se sirven con `serve`.

Observations:
- `POST /observations`
- `GET /observations`
- `GET /observations/{id}`
- `PATCH /observations/{id}`
- `DELETE /observations/{id}`

Search:
- `GET /search?query=...&project=...&topic_key=...&tags=tag1,tag2&include_deleted=true`

Topics:
- `PUT /topics/{topic_key}`

Sessions:
- `POST /sessions`
- `POST /sessions/{id}/resume`
- `PATCH /sessions/{id}`

## MCP tools
El MCP server expone los siguientes tools via JSON-RPC:
- `observation.create`
- `observation.read`
- `observation.update`
- `observation.list`
- `observation.delete`
- `search`
- `topic.upsert`
- `session.open`
- `session.resume`
- `session.update_disclosure`
- `config.show`

## OpenCode MCP plugin
Este repo incluye un OpenCode MCP plugin package para NeaBrain:
- Package: `plugins/opencode-mcp`
- Install y usage: `docs/opencode-mcp.md`
- Adapter: `plugins/opencode-mcp/adapter.ts` (registra `nbn_*` tool aliases y compaction hooks)

## Verification

Tests:

```bash
go test ./...
```

End-to-end smoke test (CLI, HTTP, MCP):

```powershell
./scripts/e2e_smoke.ps1
```
