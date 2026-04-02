# NeaBrain: Memoria Persistente para Agentes IA

NeaBrain es un sistema de memoria local-first diseñado para agentes IA. Un único binario Go con SQLite + FTS5, expuesto via CLI, HTTP API, MCP server y TUI interactivo. Sin servidores externos, sin dependencias de plataforma.

---

## Arquitectura

NeaBrain sigue una arquitectura hexagonal (ports and adapters). El core domain no importa ningún adapter: las reglas de negocio son estables mientras evoluciona el storage o se agregan nuevas interfaces.

```
cmd/neabrain/
internal/
  domain/          ← entidades, servicios, políticas de dedupe
  ports/           ← interfaces inbound y outbound
  adapters/
    inbound/
      cli/         ← CLI (flag stdlib)
      http/        ← REST API (net/http)
      mcp/         ← MCP JSON-RPC server
      tui/         ← TUI interactivo (Bubble Tea)
    outbound/
      sqlite/      ← repositorios SQLite + FTS5
  sync/            ← chunks JSONL.gz inmutables
  setup/           ← instalación de config por agente
  version/         ← check de versión via GitHub API
  infrastructure/  ← migrations SQLite
  observability/   ← logger y métricas
plugins/
  opencode-mcp/    ← plugin TypeScript para OpenCode
```

### Entidades core

| Entidad | Descripción |
|---|---|
| `Observation` | Unidad de memoria. Tiene contenido, proyecto, topic, tags, source y metadata. |
| `Topic` | Agrupación nombrada de observations. Se identifica por `topic_key` estable. |
| `Session` | Contexto de trabajo de una sesión de agente. Tiene `disclosure_level`. |
| `Duplicate` | Registro de relación dedupe entre observations. |

---

## Storage

- **SQLite** con modo WAL para lecturas concurrentes
- **FTS5** para full-text search sin dependencias externas
- **Pure Go SQLite** (`modernc.org/sqlite`) — sin CGo, sin librerías de plataforma
- Un solo archivo `.db` — fácil de respaldar, versionar o migrar

### Ubicación por defecto

```
os.UserConfigDir()/neabrain/neabrain.db
```

Override via flag `--storage-path` o variable `NEABRAIN_STORAGE_PATH`.

---

## MCP Server

NeaBrain implementa el protocolo MCP (Model Context Protocol) sobre JSON-RPC stdio. Los agentes lo conectan como proceso hijo.

```bash
neabrain mcp                    # perfil agent (default)
neabrain mcp --profile admin    # incluye herramientas destructivas
neabrain mcp --profile all      # todos los tools
```

### Perfiles

| Perfil | Descripción |
|---|---|
| `agent` | Tools de lectura y escritura útiles para agentes. Sin acceso a operaciones administrativas. |
| `admin` | Todos los tools del perfil agent más los marcados como destructivos/admin. |
| `all` | Alias de `admin`. |

### Tool annotations (MCP 2024-11-05)

Cada tool declara hints para el cliente:

| Hint | Significado |
|---|---|
| `readOnlyHint` | No modifica el estado. |
| `idempotentHint` | Llamar múltiples veces con los mismos args produce el mismo resultado. |
| `destructiveHint` | Modifica o elimina datos. Usar con cuidado. |

---

## MCP Tools — Referencia completa

### Observations

#### `nbn_observation_create` / `observation.create`
Persiste una observation. Incluir qué pasó, por qué importa y dónde en el codebase.

```json
{
  "content": "string (requerido)",
  "project": "string",
  "topic_key": "string",
  "tags": ["string"],
  "source": "string",
  "metadata": {},
  "allow_duplicate": false
}
```

> Política de dedupe por defecto: `exact`. Si el mismo contenido ya existe en el mismo proyecto, devuelve `conflict`. Usar `allow_duplicate: true` para forzar la creación.

---

#### `nbn_observation_read` / `observation.read`
Lee el contenido completo de una observation por ID. Usar después de search cuando el preview está truncado.

```json
{
  "id": "string (requerido)",
  "include_deleted": false
}
```

---

#### `nbn_observation_update` / `observation.update`
Actualiza campos de una observation existente. Solo se modifican los campos enviados.

```json
{
  "id": "string (requerido)",
  "content": "string",
  "project": "string",
  "topic_key": "string",
  "tags": ["string"],
  "source": "string",
  "metadata": {}
}
```

---

#### `nbn_observation_list` / `observation.list`
Lista observations con filtros opcionales. Devuelve objetos completos.

```json
{
  "project": "string",
  "topic_key": "string",
  "tags": ["string"],
  "include_deleted": false
}
```

---

#### `nbn_observation_delete` / `observation.delete`
Soft-delete de una observation. Sigue siendo consultable con `include_deleted: true`.

```json
{
  "id": "string (requerido)"
}
```

---

### Búsqueda

#### `nbn_search` / `search`
Full-text search sobre todas las observations. Devuelve resultados rankeados por relevancia.

```json
{
  "query": "string (requerido)",
  "project": "string",
  "topic_key": "string",
  "tags": ["string"],
  "include_deleted": false
}
```

---

#### `nbn_context`
Recupera contexto relevante para la tarea actual. Semánticamente equivalente a `nbn_search` pero orientado a recuperación de contexto antes de empezar trabajo.

```json
{
  "query": "string (requerido)",
  "project": "string",
  "topic_key": "string",
  "tags": ["string"],
  "include_deleted": false
}
```

> **Cuándo usar**: llamar `nbn_context` al inicio de cada sesión para recuperar decisiones previas y patrones del proyecto.

---

### Topics

#### `nbn_topic_upsert` / `topic.upsert`
Crea o actualiza un topic nombrado. Los topics agrupan observations por dominio (ej. `auth`, `database-schema`). Usar un `topic_key` estable entre sesiones.

```json
{
  "topic_key": "string (requerido)",
  "name": "string",
  "description": "string",
  "metadata": {}
}
```

---

### Sessions

#### `nbn_session_open` / `session.open`
Abre una nueva sesión para rastrear un contexto de trabajo.

```json
{
  "disclosure_level": "string (requerido)"
}
```

`disclosure_level` valores recomendados: `"low"`, `"medium"`, `"high"`.

---

#### `nbn_session_resume` / `session.resume`
Reanuda una sesión previamente abierta por ID.

```json
{
  "id": "string (requerido)"
}
```

---

#### `nbn_session_update_disclosure` / `session.update_disclosure`
Cambia el nivel de disclosure de una sesión abierta.

```json
{
  "id": "string (requerido)",
  "disclosure_level": "string (requerido)"
}
```

---

#### `nbn_session_summary`
Almacena un resumen estructurado de fin de sesión. Llamar en el momento de compactación. Diseñado para captura automática con las siguientes secciones:

```
## Goal
## Key Decisions
## Discoveries
## Files Changed
## Accomplished
```

```json
{
  "summary": "string (requerido)",
  "project": "string",
  "topic_key": "string",
  "tags": ["string"],
  "metadata": {}
}
```

> Tags por defecto: `["session_summary"]`

---

### Captura pasiva

#### `nbn_capture_passive`
Extrae y almacena un aprendizaje del texto sin estructuración explícita. Usar cuando se nota algo que vale la pena preservar en medio de una conversación.

```json
{
  "text": "string (requerido)",
  "project": "string",
  "topic_key": "string",
  "tags": ["string"]
}
```

> Tags por defecto: `["passive"]`

---

### Proyectos

#### `nbn_projects_list`
Lista todos los proyectos activos con sus conteos de observations.

```json
{}
```

---

#### `nbn_projects_rename`
Renombra un proyecto en todas sus observations.

```json
{
  "from": "string (requerido)",
  "to": "string (requerido)"
}
```

---

### Export

#### `nbn_export`
Exporta observations como array JSON. Útil para backup, migración o compartir memoria entre entornos.

```json
{
  "project": "string",
  "topic_key": "string",
  "tags": ["string"],
  "include_deleted": false
}
```

---

### Config

#### `nbn_config_show` / `config.show`
Muestra la configuración efectiva: storage paths, proyecto por defecto, política de dedupe.

```json
{}
```

---

### Admin tools (solo perfiles `admin` / `all`)

#### `nbn_stats`
Devuelve conteos agregados: observations activas, soft-deleted y proyectos distintos.

```json
{}
```

---

#### `nbn_merge_projects`
Fusiona múltiples variantes de nombre de proyecto en uno destino. Todas las observations de los proyectos `from` se reasignan a `to`. **Irreversible.**

```json
{
  "from": ["string (requerido)"],
  "to": "string (requerido)"
}
```

---

## Protocolo de sesión recomendado

Los agentes deberían seguir este protocolo para maximizar la utilidad de la memoria:

1. **Al inicio** — llamar `nbn_context` con el tema de la tarea para recuperar contexto previo.
2. **Durante el trabajo** — usar `nbn_capture_passive` para preservar aprendizajes emergentes.
3. **Al finalizar** — llamar `nbn_session_summary` con un resumen estructurado usando las secciones Goal / Key Decisions / Discoveries / Files Changed.

---

## Integración con agentes

### Claude Code

```bash
neabrain setup claude-code --install
```

Escribe en `~/.claude/settings.json` bajo `mcpServers`. Para hacer el proceso manual:

```json
{
  "mcpServers": {
    "neabrain": {
      "type": "stdio",
      "command": "/ruta/a/neabrain",
      "args": ["mcp"]
    }
  }
}
```

Guía completa: `docs/integrations/claude-code.md`

---

### Cursor

```bash
neabrain setup cursor --install
```

Escribe en `~/.cursor/mcp.json`.

Guía completa: `docs/integrations/cursor.md`

---

### VS Code

```bash
neabrain setup vscode --install
```

Escribe en el mcp.json del directorio de usuario de VS Code.

Guía completa: `docs/integrations/vscode.md`

---

### OpenCode

```bash
neabrain setup opencode --install
```

Escribe en `~/.config/opencode/config.json`. También disponible el plugin TypeScript en `plugins/opencode-mcp/`.

---

### Desinstalar

```bash
neabrain setup <agente> --uninstall
```

---

## Sync

Sincronización sin conflictos entre máquinas via chunks JSONL.gz inmutables.

### Cómo funciona

Cada `sync export` produce un nuevo archivo comprimido identificado por su hash SHA-256. Un `manifest.json` indexa todos los chunks disponibles. Un `sync_state.json` local registra qué chunks ya fueron importados en esta máquina.

```
~/.config/neabrain/sync/
  manifest.json        ← índice compartido (append-only)
  sync_state.json      ← estado local de esta máquina
  chunks/
    <sha256>.jsonl.gz  ← un archivo por export
```

El directorio de sync puede vivir en git, Dropbox, o cualquier sistema de archivos compartido. Nunca hay merge conflicts porque los chunks son inmutables.

### Comandos

```bash
# Exportar todas las observations como nuevo chunk
neabrain sync export

# Exportar solo un proyecto
neabrain sync export --project "mi-proyecto"

# Ver estado: cuántos chunks existen y cuántos faltan importar
neabrain sync status

# Importar todos los chunks pendientes (idempotente)
neabrain sync import

# Usar un directorio custom
neabrain sync export --dir /ruta/dropbox/neabrain-sync
```

### Write notifications

El servidor HTTP puede disparar un export automático después de cada mutación:

```bash
neabrain serve --sync-dir ~/.config/neabrain/sync
```

---

## CLI — Referencia completa

```
neabrain observation create  --content "..." [--project P] [--topic T] [--tags a,b] [--source S]
neabrain observation read    --id ID
neabrain observation update  --id ID [--content "..."] [--project P]
neabrain observation list    [--project P] [--topic T] [--tags a,b]
neabrain observation delete  --id ID
neabrain observation export  [--project P] [--output archivo.json]
neabrain observation import  <archivo.json>

neabrain search              --query "..." [--project P]

neabrain projects list
neabrain projects rename     --from "viejo" --to "nuevo"

neabrain topic upsert        --topic-key "auth" --name "Auth" --description "..."

neabrain session open        --disclosure-level low
neabrain session resume      --id ID
neabrain session update-disclosure --id ID --level high

neabrain config show

neabrain setup <claude-code|cursor|vscode|opencode> [--install] [--uninstall]

neabrain version
neabrain version --check

neabrain sync export         [--project P] [--dir D]
neabrain sync import         [--dir D]
neabrain sync status         [--dir D]

neabrain serve               [--addr :8080] [--sync-dir D]
neabrain mcp                 [--profile agent|admin|all]
neabrain tui
```

### Flags de configuración globales

| Flag | Variable de entorno | Descripción |
|---|---|---|
| `--storage-path` | `NEABRAIN_STORAGE_PATH` | Ruta del archivo SQLite |
| `--fts-path` | `NEABRAIN_FTS_PATH` | Ruta del índice FTS5 |
| `--default-project` | `NEABRAIN_DEFAULT_PROJECT` | Proyecto por defecto |
| `--dedupe-policy` | `NEABRAIN_DEDUPE_POLICY` | `exact` (default) o `none` |
| `--config-file` | `NEABRAIN_CONFIG_FILE` | Ruta del archivo de configuración |

---

## HTTP API

```bash
neabrain serve --addr :8080
```

| Método | Ruta | Descripción |
|---|---|---|
| `POST` | `/observations` | Crear observation |
| `GET` | `/observations` | Listar observations |
| `GET` | `/observations/{id}` | Leer observation |
| `PATCH` | `/observations/{id}` | Actualizar observation |
| `DELETE` | `/observations/{id}` | Soft-delete |
| `GET` | `/search?query=...` | Full-text search |
| `PUT` | `/topics/{key}` | Upsert topic |
| `POST` | `/sessions` | Abrir sesión |
| `POST` | `/sessions/{id}/resume` | Reanudar sesión |
| `PATCH` | `/sessions/{id}` | Actualizar sesión |

Query params de `/search`: `query`, `project`, `topic_key`, `tags`, `include_deleted`.

---

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

Verificar instalación:

```bash
neabrain version
```

---

## Releases

| Versión | Cambios principales |
|---|---|
| [v0.4.0](https://github.com/RDuuke/neabrain/releases/tag/v0.4.0) | Sync JSONL.gz chunks, write notifications en HTTP |
| [v0.3.0](https://github.com/RDuuke/neabrain/releases/tag/v0.3.0) | `neabrain version --check`, `setup --install/--uninstall` |
| [v0.2.0](https://github.com/RDuuke/neabrain/releases/tag/v0.2.0) | Export/import JSON, projects, AGENTS.md, perfiles MCP, TUI Bubble Tea |
