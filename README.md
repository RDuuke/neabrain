# NeaBrain

NeaBrain is a single-user memory system that delivers consistent behavior across CLI, HTTP, MCP, and TUI interfaces. The architecture isolates core domain logic from adapter and storage concerns so all interfaces share identical semantics.

## Goals
- Provide consistent CRUD, search, dedupe, topic upsert, and session behavior across adapters.
- Keep domain rules stable while allowing storage, FTS, and interface implementations to evolve.
- Support local-first operation with clear configuration and override precedence.

## Architecture
NeaBrain follows a hexagonal (ports and adapters) architecture:
- **Core entities**: Observation, Topic, Session, Duplicate.
- **Inbound ports**: ObservationService, SearchService, TopicService, SessionService, ConfigService.
- **Outbound ports**: ObservationRepository, TopicRepository, SessionRepository, SearchIndex, Clock.
- **Adapters**: CLI, HTTP, MCP, TUI; plus local storage, FTS, config, and clock implementations.

## Operations
Configuration defaults, environment variables, and verification steps are documented in `docs/operations.md`.
