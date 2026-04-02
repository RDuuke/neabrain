# NeaBrain

![NeaBrain Logo](logo.png)

Sistema de memoria persistente local-first para agentes IA. Expone las mismas operaciones por CLI, HTTP, MCP y TUI. Arquitectura hexagonal: el domain core no depende de ningún adapter.

## Instalación

```bash
go install github.com/RDuuke/neabrain/cmd/neabrain@latest
```

O desde source (Go 1.22+):

```bash
git clone https://github.com/RDuuke/neabrain.git
cd neabrain
go build -o neabrain ./cmd/neabrain
```

## Inicio rápido

```bash
# Guardar una observación
neabrain observation create --content "usar context.WithTimeout en MCP" --project "nea-brain" --tags "go,mcp"

# Buscar
neabrain search --query "timeout" --project "nea-brain"

# Ver proyectos
neabrain projects list

# Exportar todas las observaciones como JSON
neabrain observation export --output backup.json

# Sincronizar entre máquinas
neabrain sync export
neabrain sync status
```

## Integración con agentes IA

NeaBrain funciona como servidor MCP. Conecta tu agente preferido con un solo comando:

```bash
# Ver el snippet de configuración
neabrain setup claude-code
neabrain setup cursor
neabrain setup vscode
neabrain setup opencode

# O escribir la configuración directamente
neabrain setup claude-code --install
neabrain setup claude-code --uninstall
```

Guías detalladas en `docs/integrations/`.

### MCP tools disponibles

| Tool | Descripción |
|---|---|
| `nbn_capture` | Crea una observación |
| `nbn_capture_passive` | Captura pasiva (sin confirmación) |
| `nbn_read` | Lee una observación por ID |
| `nbn_timeline` | Navega contexto cronológico alrededor de una observación |
| `nbn_update` | Actualiza una observación |
| `nbn_list` | Lista observaciones con filtros |
| `nbn_delete` | Soft-delete |
| `nbn_search` | Búsqueda full-text |
| `nbn_topic_upsert` | Upsert de topic |
| `nbn_topics_list` | Lista topics con counts |
| `nbn_session_open` | Abre sesión |
| `nbn_session_resume` | Reanuda sesión |
| `nbn_stats` | Estadísticas del store |
| `nbn_projects_list` | Lista proyectos |
| `nbn_projects_rename` | Renombra proyecto |
| `nbn_sync_status` | Muestra estado del sync dir |
| `nbn_sync_export` | Exporta snapshot actual a un chunk |
| `nbn_sync_import` | Importa chunks pendientes |
| `nbn_merge_projects` | Fusiona proyectos |
| `nbn_config_show` | Muestra configuración activa |

#### Perfiles MCP

```bash
neabrain mcp                    # perfil agent (solo lectura/captura)
neabrain mcp --profile admin    # incluye herramientas destructivas
neabrain mcp --profile all      # todas las herramientas
```

## CLI — referencia

```
neabrain observation <create|read|timeline|update|list|delete|export|import>
neabrain search [--query Q] [--project P]
neabrain topic <upsert|list>
neabrain session <open|resume|update-disclosure>
neabrain projects <list|rename>
neabrain config show
neabrain setup <claude-code|cursor|vscode|opencode> [--install] [--uninstall]
neabrain version [--check]
neabrain sync <export|import|status> [--dir D] [--project P]
neabrain serve [--addr :8080] [--sync-dir D]
neabrain mcp [--profile agent|admin|all]
neabrain tui
```

Flags de configuración disponibles en todos los comandos:

| Flag | Variable de entorno | Descripción |
|---|---|---|
| `--storage-path` | `NEABRAIN_STORAGE_PATH` | Ruta del archivo SQLite |
| `--fts-path` | `NEABRAIN_FTS_PATH` | Ruta del índice FTS |
| `--default-project` | `NEABRAIN_DEFAULT_PROJECT` | Proyecto por defecto |
| `--dedupe-policy` | `NEABRAIN_DEDUPE_POLICY` | `exact` (default) o `none` |
| `--config-file` | `NEABRAIN_CONFIG_FILE` | Ruta del archivo de configuración |

## HTTP API

```bash
neabrain serve --addr :8080
```

| Método | Ruta | Descripción |
|---|---|---|
| `POST` | `/observations` | Crear observación |
| `GET` | `/observations` | Listar observaciones |
| `GET` | `/observations/{id}` | Leer observación |
| `PATCH` | `/observations/{id}` | Actualizar observación |
| `DELETE` | `/observations/{id}` | Soft-delete |
| `GET` | `/search?query=...` | Búsqueda full-text |
| `PUT` | `/topics/{key}` | Upsert de topic |
| `POST` | `/sessions` | Abrir sesión |
| `POST` | `/sessions/{id}/resume` | Reanudar sesión |
| `PATCH` | `/sessions/{id}` | Actualizar sesión |

Con `--sync-dir`, el servidor dispara un export automático en background después de cada escritura.

## Sync

Sincronización portable y sin conflictos entre máquinas via chunks JSONL.gz inmutables:

```
~/.config/neabrain/sync/
  manifest.json        ← índice compartido (append-only, git-friendly)
  sync_state.json      ← qué chunks importó esta máquina
  chunks/
    <sha256>.jsonl.gz  ← cada export produce un chunk nuevo
```

```bash
# Exportar observaciones como nuevo chunk
neabrain sync export --project "mi-proyecto"

# Ver estado
neabrain sync status

# Importar chunks pendientes (idempotente)
neabrain sync import
```

El mismo directorio puede vivir en Dropbox, un repo git, o cualquier sistema de archivos compartido.

Los mismos flujos de sync también están disponibles por MCP con `nbn_sync_status`,
`nbn_sync_export` y `nbn_sync_import`.

## Configuración

Ruta por defecto: `os.UserConfigDir()/neabrain/config.json`

Precedencia: flags CLI > variables de entorno > archivo de config > defaults

## Arquitectura

```
cmd/neabrain/
internal/
  domain/          ← entidades, servicios, políticas (sin dependencias externas)
  ports/           ← interfaces inbound y outbound
  adapters/
    inbound/
      cli/         ← CLI (flag)
      http/        ← HTTP API (net/http)
      mcp/         ← MCP JSON-RPC server
      tui/         ← TUI (Bubble Tea)
    outbound/
      sqlite/      ← repositorios SQLite + FTS5
  sync/            ← sync de chunks JSONL.gz
  setup/           ← instalación de config por agente
  version/         ← check de versión via GitHub API
  infrastructure/  ← migrations SQLite
  observability/   ← logger y métricas
plugins/
  opencode-mcp/    ← plugin TypeScript para OpenCode
docs/
  integrations/    ← guías por agente (claude-code, cursor, vscode)
```

## Tests

```bash
go test ./...
```

## Releases

- [v0.4.0](https://github.com/RDuuke/neabrain/releases/tag/v0.4.0) — sync de chunks, write notifications
- [v0.3.0](https://github.com/RDuuke/neabrain/releases/tag/v0.3.0) — version check, setup --install
- [v0.2.0](https://github.com/RDuuke/neabrain/releases/tag/v0.2.0) — export/import, projects, AGENTS.md, MCP profiles, TUI Bubble Tea
