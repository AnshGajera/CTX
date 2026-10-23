# CTX — Context Engine for Software Development

> Git for Context, not Code. Extract structured project context (architecture, APIs, DB schemas, deps, env, patterns) and serve it to AI tools via MCP. Hybrid TF-IDF + MiniLM retrieval for RAG. Local dashboard, health scoring, and a self-hosted team server included.

**New users:** follow the [CTX User Guide](./USER_GUIDE.md) for installation,
first-project setup, AI/MCP integration, optional ML services, and daily usage.

## Install

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/AnshGajera/CTX/master/scripts/install.sh | bash
# Windows (PowerShell)
powershell -ExecutionPolicy Bypass -File scripts/install.ps1
# From source
go build -o ctx ./cmd/ctx
```

## Quickstart

```bash
ctx init       # interactive wizard: banner, project name, section toggles, MCP editor setup
ctx extract
ctx status
ctx health     # context quality score (0-100) with tips
ctx search "auth flow"
ctx export --format openapi -o openapi.json
ctx serve --ui             # dashboard at http://127.0.0.1:3100/ui
ctx serve --stdio          # MCP for Cursor / Claude Desktop / VS Code
ctx dashboard              # terminal task menu (extract/search/diff/history/serve/export)

# Git Context Control (GCC)
ctx branch feature-auth    # create a context branch
ctx checkout feature-auth  # switch context branch
ctx commit -m "auth done"  # save a reasoning checkpoint
ctx merge feature-auth     # merge context branches
ctx tag v1.0.0             # tag a context milestone
ctx log                    # history with branch/tag decorations

# Version management
ctx bump current           # show current version
ctx bump patch             # 0.1.5 -> 0.1.6
ctx bump minor --tag       # 0.1.5 -> 0.2.0 + git tag v0.2.0
ctx bump major --dry-run   # preview major bump without writing
```

Non-interactive (CI/scripts): `ctx init --yes` skips the wizard; `--editor cursor|claude-desktop|vscode|none` and `--sections api_endpoints` preselect wizard answers. `--json` on init/extract/status/diff/log/search/eval/health gives machine-readable output.

## Five-minute demo

Run this from a project that contains source code:

```bash
ctx init --yes --editor none
ctx extract
ctx status
ctx health
ctx search "authentication"
ctx serve --ui
```

Open `http://127.0.0.1:3100/ui` to show the extracted architecture, APIs, database models, environment requirements, search, and snapshot history. In a second terminal, run `ctx serve --stdio` to connect an MCP client.

## What is implemented

| Area | Status |
|---|---|
| Local extraction, privacy sanitization, snapshots, branches, tags, diff, merge | Implemented |
| CLI search with TF-IDF and optional MiniLM sidecar | Implemented |
| Dashboard, REST context endpoints, and MCP stdio transport | Implemented |
| Self-hosted SQLite/JWT server with login, organizations, projects, push, pull, history, and API tokens | Implemented |
| HTTP MCP transport and hosted cloud backend | Not included; use stdio and the self-hosted server |

## Commands

| Command | Description |
|---|---|
| `ctx init [--name]` | Detect stack, write `.ctx/`, install git hook |
| `ctx extract` | Run extractors, sanitize, save + snapshot (skips when unchanged) |
| `ctx status` | Counts, context branch, diff vs parent |
| `ctx diff [h1] [h2]` | Colored snapshot diff (supports `HEAD~N`, branch, tag refs) |
| `ctx log` | Snapshot history with branch/tag decorations |
| `ctx search <q>` | Hybrid semantic search over context |
| `ctx eval` | Retrieval `hit@k` self-eval |
| `ctx health` | Context quality score (0-100) with tips |
| `ctx dashboard` | Terminal task menu (extract/search/diff/history/serve/export) |
| `ctx export --format openapi` | Export endpoints as OpenAPI 3.0 |
| `ctx serve [--ui] [--bind]` | MCP server (HTTP + stdio) + embedded web dashboard |
| `ctx ui [--port] [--bind]` | Compatibility alias for `ctx serve --ui` |
| `ctx watch` | File watcher with debounce (single-flight re-extract) |
| `ctx push/pull/share/login` | Team sync via `ctx-server` (self-hosted, JWT) |
| `ctx team login/push/pull` | Documentation-compatible grouped aliases for team sync |
| **Git Context Control (GCC)** | |
| `ctx branch [name]` | List, create (`ctx branch feat`), or delete (`-d feat`) context branches |
| `ctx checkout <target>` | Switch context branch or restore snapshot (`-b` creates new branch) |
| `ctx commit -m "msg"` | Record explicit context checkpoint / reasoning milestone |
| `ctx merge <branch>` | Merge context from another branch (reconciles APIs, models, env, deps) |
| `ctx tag [name]` | Tag context snapshots as milestones (`-d` to delete, `-l` to list) |
| **Version management** | |
| `ctx bump [patch\|minor\|major]` | Bump semver across `version.go`, `_version.py` (supports `--tag`, `--dry-run`) |

## MCP integration (stdio-first)

**Recommended: stdio** — full MCP protocol (JSON-RPC), works with Cursor, Claude Desktop, and any MCP client:

```json
{"mcpServers": {"ctx": {"command": "ctx", "args": ["serve", "--stdio"]}}}
```

`ctx init` can write this for you: choose `cursor` (writes `.cursor/mcp.json`), `vscode` (writes `.vscode/mcp.json`), or `claude-desktop` (merges into the Claude Desktop user config, preserving your other servers).

For compatibility with older website examples, `ctx serve --mcp` is accepted as
an alias for `ctx serve --stdio`. The canonical command remains
`ctx serve --stdio`.

**HTTP mode** (`ctx serve --port 3100`) is a plain REST API for scripts and debugging (`/context`, `/context/apis`, `/mcp/tools/*`) — it is *not* the MCP Streamable HTTP protocol, so point MCP clients at stdio. Binds `127.0.0.1` by default (`--bind 0.0.0.0` only on trusted networks), with server timeouts and 5MB body caps.

Tools: `get_project_context` (supports `sections`, `max_tokens`), `get_context_for_task`, `get_context_for_file`, `get_api_endpoints`, `get_database_schema`, `get_project_conventions`, `get_env_requirements`, `search_context`.

Legacy aliases are also published for existing integrations:
`ctx_search`, `ctx_get_schema`, `ctx_inspect_route`, and
`ctx_get_env_vars`.

GCC tools (available via MCP stdio and HTTP): `branch_context` (list/create/delete context branches), `checkout_context` (switch active branch), `commit_context` (save reasoning checkpoint), `merge_context` (reconcile branches), `tag_context` (milestone tagging), `get_context_diff` (structured diff between refs), `get_context_history` (timeline with branch/tag metadata).

Tip: `GET /context?max_tokens=4000` returns budget-truncated context (least-important sections dropped first) so large repos fit model windows.

## Git Context Control (GCC)

Inspired by the [Git-Context-Controller](https://arxiv.org/abs/2508.00031) framework, CTX treats context as a versioned, branching file system — giving AI agents structured long-term memory with `COMMIT`, `BRANCH`, `MERGE`, and `CONTEXT` operations.

**Branches** let agents explore alternative reasoning paths or sub-tasks without polluting the main context. **Checkpoints** (`ctx commit`) save explicit milestones with descriptions. **Merge** reconciles context discovered on different branches (APIs, models, env vars, deps, patterns) with intelligent deduplication. **Tags** mark release baselines or milestone snapshots.

Context state is stored under `.ctx/refs/heads/` (branches) and `.ctx/refs/tags/` (tags), with a git-style `HEAD` file (symbolic ref or detached hash). All existing commands (`diff`, `log`, `status`, `serve`, `search`) work seamlessly with the active branch.

```bash
ctx branch                         # list branches (* marks current)
ctx checkout -b explore-auth       # create and switch to new branch
ctx extract                        # extract on the new branch
ctx commit -m "auth endpoints discovered"  # save milestone
ctx checkout main                  # switch back
ctx merge explore-auth             # integrate discoveries
ctx tag v1.0.0-alpha               # tag the merged state
ctx log                            # see decorated history
```

## Version bump (`ctx bump`)

`ctx bump` provides semver version management for Go projects, auto-detecting the current version from `_version.py`, `internal/version/version.go`, or git tags, and bumping it across all version files in one command.

```bash
ctx bump current                  # display current version
ctx bump patch                    # 0.1.5 -> 0.1.6, updates version.go + _version.py
ctx bump minor --tag              # 0.1.5 -> 0.2.0, creates git tag v0.2.0
ctx bump major --dry-run          # preview 0.1.5 -> 1.0.0 without writing
ctx bump 2.0.0-rc.1 --tag         # set explicit version + tag
ctx bump patch --file pkg/ver.go  # include additional version files
```

Supports `--json` for CI integration. `--dry-run` previews changes without modifying files.

## Dashboard + health + freshness

`ctx serve --ui` serves an offline single-page dashboard at `/ui` (overview, endpoint table with filters, schema + ER diagram, env, deps, history/diff timeline, search box). Extra API for scripts: `/api/history`, `/api/diff?from=&to=` (hash allowlisted), `/api/search?q=`.

`ctx health` scores context 0–100 (grade A–D): endpoint docs, architecture pattern, DB diagram, env descriptions, business rules, freshness, patterns, key files, deps, structure, model fields — with `💡` tips. Every extracted section carries `section_meta` provenance (`source: regex|ast|regex+ast`, `confidence`, `extractors`, `item_count`) and staleness (`fresh` <1d, `stale` <7d, `expired` beyond), also exposed in `/context/summary` so AI agents can prefer fresh context.

## Supported stack matrix

| Area | Coverage |
|---|---|
| Languages | TypeScript/JavaScript, Go, Python, Rust, Java/Kotlin (detection); TS/JS, Go, Python (route extraction) |
| Frameworks | Next.js (App + Pages router), Express/Fastify/Hono, Gin/Echo/Fiber/Chi/Mux, Flask/FastAPI/Django |
| APIs | REST route extraction + GraphQL schema/queries/mutations/subscriptions (`.graphql`/`.gql`) |
| Database | Prisma (full schema + ER diagram), Mongoose, TypeORM, SQLAlchemy, Django ORM, GORM, plus migration files |
| Env | `.env.example` + code refs (`process.env`, `os.getenv`, …) + compose + Dockerfile; values never stored |
| Precision mode | `ctx-ml` sidecar: Python `ast` + tree-sitter TS route extraction merges what regex missed |

## ML sidecar (`ctx-ml/`)

```bash
pip install -r ctx-ml/requirements.txt          # base: server + AST + tests (no torch)
pip install -r ctx-ml/requirements-ml.txt       # optional: MiniLM embeddings (~2GB)
uvicorn ctx-ml.app:app --port 8001
# or: docker compose up ctx-ml
python ctx-ml/eval.py .ctx/context.json 5
```

Uses `sentence-transformers/all-MiniLM-L6-v2` when installed, TF-IDF fallback otherwise (Go and Python rankers are parity-tested). `ctx extract` auto-merges sidecar AST routes when the sidecar is reachable; offline it silently falls back to regex extractors. Sidecar guards: `top_k` clamped to 1–50, 413 over 20k chunks, 1MB/20k-file scan caps.

## Team server (`server/`)

Self-hosted sync backend (SQLite + JWT, defaults `127.0.0.1:3200`):

```bash
go run ./server/cmd/ctx-server --help   # -bind/-port/-db/-jwt-secret, or CTX_* env
# or: CTX_JWT_SECRET="$(openssl rand -hex 32)" docker compose up ctx-server
# Windows PowerShell: $env:CTX_JWT_SECRET = [guid]::NewGuid().ToString("N"); docker compose up ctx-server
```

Point the CLI at it via `core.api_url`, then `ctx login` (`--token` skips the interactive password prompt; `CTX_TOKEN` env also works), `ctx push/pull`, `ctx share --create-token`. Orgs/projects/history/share/token endpoints live under `/api/v1/*` (10MiB/100-snapshot caps). Authentication requires an `Authorization: Bearer` header; tokens in query strings are rejected to avoid leaking credentials through logs and browser history. Pulled snapshots are hash-verified before touching disk.

The grouped compatibility form is also supported:

```bash
ctx team login http://127.0.0.1:3200 --token "$CTX_TOKEN"
ctx team push -m "initial context"
ctx team pull
```

The local context store is file-based (`.ctx/context.json`, snapshots, refs,
and manifest metadata); CTX does not create a `.ctx/index.db` file. SQLite is
used by the optional self-hosted team server.

### Complete team-server setup

```bash
# Start the server with a secret (copy .env.example and replace the value).
cp .env.example .env
export CTX_JWT_SECRET="$(openssl rand -hex 32)"
docker compose up -d ctx-server

# Register through the API.
curl -X POST http://127.0.0.1:3200/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","name":"Demo User","password":"correct horse battery staple"}'

# Configure the project after creating an organization and project through the API.
# Set core.api_url = "http://127.0.0.1:3200" in .ctx/config.toml.
ctx login
ctx push -m "initial context"
ctx pull
```

The server image requires CGO only while building the Go binary; the runtime image contains SQLite libraries and a health check. Docker Compose requires `CTX_JWT_SECRET` explicitly and stores server data in the `ctx-data` volume.

## Privacy

Never stores `.env` values — only names, categories, required flags. Sensitive defaults redacted (`[REDACTED]`), plus entropy + known-pattern scanning of free text (TODOs, examples, rule/descriptions) and secret-looking values under generic names. HTTPS enforced for non-local `api_url`. `.ctxignore` supports gitignore-style patterns (`*.log`, `secrets/`, `**`, `!` negation) and prunes whole ignored directories.

## Config (`~/.config/ctx/config.toml` + `.ctx/config.toml`)

`core.api_url/ml_url`, `extraction.*` toggles, `privacy.*`, `sync.*`. `ml_url = ""` disables the sidecar. Installers verify release checksums and abort when missing.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `not initialized (run ctx init)` | Run `ctx init` in the project root first |
| `ctx diff` → `no parent at HEAD~1` | Only one snapshot exists; run `ctx extract` after changing code |
| `search` returns nothing | Run `ctx extract` first; check `.ctx/context.json` exists |
| `extract` always says "No changes" | Correct — snapshots dedupe on identical content; edit code to get a new snapshot |
| MCP `serve` port in use | `ctx serve --port 3200` (server binds `127.0.0.1`; `--bind` to change) |
| `ctx health` score low | Follow the `💡` tips (docs, `.env.example` comments, `docs/decisions/` ADRs) |
| `ctx login` password prompt | Password input is hidden on interactive terminals; use `ctx login --token` or `CTX_TOKEN` for automation |
| Sidecar AST adds nothing | Start it (`uvicorn ctx-ml.app:app --port 8001`); Go falls back silently when down |
| `go: no required module` | Module is `github.com/AnshGajera/CTX`; run `go mod tidy` |

## Contributing / License

MIT. See `Makefile` (`build/test/test-go/test-python/lint/release`). Run `go test ./...` and `python -m pytest ctx-ml/tests` before PRs. See `CONTRIBUTING.md` and `SECURITY.md`.

Version: `0.1.5` — bump with `ctx bump patch` or `make build VERSION=0.2.0`.

## Complete project explanation

### 1. What problem CTX solves

Modern software projects contain useful knowledge in many different places:

- source code and folder structure;
- REST and GraphQL routes;
- database models and migrations;
- package manifests and dependency files;
- environment-variable references;
- coding conventions and recurring patterns;
- TODOs, documentation, and Git history.

AI coding tools usually need this information before they can answer questions
accurately. Reading the whole repository on every request is slow, expensive,
and can expose sensitive information. CTX solves this problem by creating a
sanitized, structured, searchable representation of a repository.

The central idea is:

> CTX is version control for project context, not for source code.

The output is stored in `.ctx/`, can be searched locally, exposed to an AI
through MCP, compared over time, branched, merged, tagged, and synchronized
with a self-hosted team server.

### 2. High-level architecture

```text
Developer repository
        |
        v
  ctx init / ctx extract
        |
        v
  Project detector
  (languages, frameworks, databases)
        |
        v
  Extraction engine
  +------------------+------------------+------------------+
  | Architecture     | APIs             | Database schema  |
  | Dependencies     | Environment      | Patterns/TODOs   |
  +------------------+------------------+------------------+
        |
        v
  Privacy sanitization and deterministic hashing
        |
        v
  .ctx/context.json + snapshots + refs
        |
        +--> CLI search / health / diff / export
        +--> Local dashboard and REST endpoints
        +--> MCP stdio tools for AI clients
        +--> Optional self-hosted team server
        +--> Optional Python ML sidecar
```

The Go CLI is the main application. The optional Python sidecar improves
precision for syntax-aware extraction and semantic retrieval. The separate
server module provides authenticated multi-user synchronization; it is not
required for local use.

### 3. Main repository components

| Component | Responsibility |
|---|---|
| `cmd/ctx` | Compiles the user-facing `ctx` command |
| `internal/cli` | Defines commands, flags, prompts, output, and configuration |
| `internal/detector` | Detects languages, frameworks, package managers, databases, and project type |
| `internal/extractors` | Reads source files and manifests and produces structured context |
| `internal/engine` | Runs extractors, merges results, deduplicates, sorts, and hashes context |
| `internal/context` | Defines the context data model and JSON representation |
| `internal/privacy` | Removes values and redacts secret-like content before storage |
| `internal/versioning` | Implements snapshots, branches, tags, diffs, commits, and merges |
| `internal/health` | Scores context quality and provides improvement tips |
| `internal/mcp` | Provides MCP stdio JSON-RPC tools and local HTTP/REST routes |
| `internal/openapi` | Exports extracted API endpoints as OpenAPI 3.0 |
| `internal/watcher` | Re-extracts context after debounced file changes |
| `ctx-ml/` | Optional Python AST, tree-sitter, and embedding sidecar |
| `server/` | Self-hosted SQLite/JWT API for organizations and project sync |
| `.github/workflows` | Automated Go tests, vetting, server checks, and Compose validation |

### 4. How extraction works

#### Step 1: Project detection

`ctx init` and the extraction workflow inspect the repository without
executing project code. CTX identifies:

- programming languages from file extensions;
- frameworks from dependency manifests and source signals;
- package managers such as npm, pnpm, pip, Poetry, Go modules, and Cargo;
- database technologies;
- frontend/backend combinations;
- likely project type such as `web_application`, `api_service`,
  `cli_tool`, `desktop_mobile`, or `library`.

Nested applications are supported. For example, a repository containing
`backend/requirements.txt` and `frontend/package.json` is treated as a
full-stack application rather than as an undetected library.

#### Step 2: Parallel section extraction

The extraction engine creates independent extractors for the enabled sections
and runs them in parallel. The main sections are:

1. **File structure and architecture** — important files, layers, services,
   entry points, and directory organization.
2. **API endpoints** — REST routes from FastAPI, Flask, Django, Express,
   Fastify, Hono, Go routers, and TypeScript/JavaScript frameworks.
3. **GraphQL** — schemas, queries, mutations, and subscriptions.
4. **Database schema** — Prisma, Mongoose, TypeORM, SQLAlchemy, Django ORM,
   GORM, and migration files.
5. **Dependencies** — direct and development dependencies from root and
   nested manifests.
6. **Environment requirements** — variable names, categories, required flags,
   examples, Docker references, and Compose references.
7. **Patterns and business rules** — recurring implementation patterns,
   conventions, rules, and TODOs.
8. **Git state** — current branch, recent commits, and repository status.

Each extractor returns structured Go data rather than copying source code
blindly. The engine merges overlapping results, removes duplicates, sorts them
deterministically, and computes a content hash.

#### Step 3: Precision enhancement

The default extractors are lightweight and work offline. When `ctx-ml` is
available, CTX can additionally use:

- Python's `ast` module for syntax-aware Python analysis;
- tree-sitter for TypeScript route extraction;
- MiniLM embeddings for semantic retrieval.

The sidecar is optional. If it is unavailable, CTX continues with the Go
extractors and TF-IDF retrieval.

#### Step 4: Privacy sanitization

Before context is saved:

- `.env` values are never stored;
- secret-looking values are replaced with `[REDACTED]`;
- entropy and known-secret patterns are checked in free text;
- `.ctxignore` and built-in exclusions remove ignored files/directories;
- only names, categories, descriptions, and required flags are retained for
  environment variables.

CTX is designed to describe a project without becoming a copy of the project.

#### Step 5: Snapshot creation

The sanitized context is serialized deterministically and hashed. If the hash
has not changed, CTX skips creating a duplicate snapshot. Otherwise it stores
the new state under `.ctx/`, updates the active context reference, and records
section provenance and freshness metadata.

### 5. The `.ctx/` directory

`.ctx/` is CTX's local context database:

| Path | Meaning |
|---|---|
| `.ctx/context.json` | Current structured project context |
| `.ctx/config.toml` | Project extraction, privacy, sync, and ML settings |
| `.ctx/snapshots/` | Immutable context snapshots |
| `.ctx/refs/heads/` | Context branch references |
| `.ctx/refs/tags/` | Context milestone tags |
| `.ctx/HEAD` | Current branch or detached snapshot |
| `.ctx/manifest.json` | Snapshot and repository metadata |
| `.ctx/hooks/` | Optional Git integration |

CTX context branches are separate from source-code branches conceptually.
They allow an agent or developer to explore a reasoning path, extract new
knowledge, and merge only the useful context back into the main line.

### 6. Search and AI integration

Local search combines lexical TF-IDF ranking with optional MiniLM semantic
embeddings. Search can be used from the CLI:

```bash
ctx search "how does authentication work"
```

Or through MCP tools such as:

- `get_project_context`;
- `get_context_for_task`;
- `get_context_for_file`;
- `get_api_endpoints`;
- `get_database_schema`;
- `get_project_conventions`;
- `get_env_requirements`;
- `search_context`.

The recommended AI integration is MCP over stdio because it implements the
standard JSON-RPC MCP workflow and works with Cursor, Claude Desktop, and
VS Code-compatible MCP clients. The local HTTP mode is intentionally a REST
and debugging API, not MCP Streamable HTTP.

### 7. Dashboard and health score

`ctx serve --ui` starts an offline dashboard that displays:

- project overview and detected technology;
- endpoint lists and filters;
- database models and relationship diagrams;
- environment requirements;
- dependencies and patterns;
- context history and diffs;
- search results;
- freshness and extraction provenance.

`ctx health` scores the usefulness of the context from 0 to 100. The score
considers documentation quality, API descriptions, architecture information,
database diagrams, model fields, environment explanations, dependencies,
patterns, key files, structure, and freshness. The command also returns
actionable tips, so a low score tells the developer how to improve the
repository's AI-readiness.

### 8. Team synchronization

The optional `ctx-server` is a self-hosted backend built with Go, SQLite, and
JWT authentication. It supports:

- user registration and login;
- organizations and projects;
- authenticated push and pull;
- snapshot history;
- project sharing and API tokens;
- hash-verified snapshot transfer.

The server requires an explicit `CTX_JWT_SECRET`. Authentication uses the
`Authorization: Bearer <token>` header; query-string tokens are rejected to
avoid credential leakage through logs and browser history. Snapshot uploads
validate hashes, JSON context, context size, and snapshot count.

### 9. Typical end-to-end workflow

```text
1. Developer runs ctx init
2. CTX detects the repository stack
3. CTX creates .ctx/ and optional AI-client configuration
4. Developer runs ctx extract
5. Extractors scan source and manifests
6. Privacy layer sanitizes values
7. Engine merges, deduplicates, sorts, and hashes results
8. Snapshot is saved only if context changed
9. Developer checks ctx health and ctx status
10. AI client reads context through MCP
11. Developer branches, commits, diffs, tags, or syncs context as needed
```

### 10. Review-ready demonstration

For a project review, demonstrate CTX in this order:

```bash
ctx init --yes --editor none
ctx extract
ctx status
ctx health
ctx search "authentication and database flow"
ctx export --format openapi -o openapi.json
ctx serve --ui
```

Then show the dashboard, explain that the displayed information was extracted
from the repository, and connect an MCP client with:

```json
{
  "mcpServers": {
    "ctx": {
      "command": "ctx",
      "args": ["serve", "--stdio"]
    }
  }
}
```

For a full-stack project such as ChatPDF, the important demonstration points
are nested frontend/backend detection, FastAPI endpoint metadata, authentication
signals, SQLAlchemy model extraction, dependency discovery, privacy-safe
environment analysis, health scoring, and OpenAPI export.

### 11. Current boundaries

CTX is intentionally a static analysis and context-management tool. It does
not execute the target application's business logic, guarantee complete
semantic understanding of every programming language, or replace application
tests and security review. Regex-based extraction is supplemented by the
optional ML sidecar where higher syntax precision is needed. The self-hosted
server is suitable for controlled deployments, but production deployments
should still use HTTPS, secure secret management, backups, access controls, and
regular dependency updates.

### 12. Why the project is useful

CTX turns an unfamiliar repository into an explainable, privacy-aware context
layer that both humans and AI systems can consume. It reduces repeated
repository scanning, makes project knowledge searchable, preserves context
history independently from code history, and provides a practical bridge
between local developer workflows and team-level AI-assisted development.
