# CTX — Context Engine for Software Development

> Git for Context, not Code. Extract structured project context (architecture, APIs, DB schemas, deps, env, patterns) and serve it to AI tools via MCP. Hybrid TF-IDF + MiniLM retrieval for RAG.

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
ctx search "auth flow"
ctx export --format openapi -o openapi.json
ctx serve --port 3100        # HTTP REST API
ctx serve --stdio            # MCP for Cursor / Claude Desktop / VS Code
```

Non-interactive (CI/scripts): `ctx init --yes` skips the wizard; `--editor cursor|claude-desktop|vscode|none` and `--sections api_endpoints` preselect wizard answers. `--json` on init/extract/status/diff/log/search/eval gives machine-readable output.

## Commands

| Command | Description |
|---|---|
| `ctx init [--name]` | Detect stack, write `.ctx/`, install git hook |
| `ctx extract` | Run extractors, sanitize, save + snapshot (skips when unchanged) |
| `ctx status` | Counts, branch, diff vs parent |
| `ctx diff [h1] [h2]` | Colored snapshot diff (supports `HEAD~N`) |
| `ctx log` | Snapshot history |
| `ctx search <q>` | Hybrid semantic search over context |
| `ctx eval` | Retrieval `hit@k` self-eval |
| `ctx export --format openapi` | Export endpoints as OpenAPI 3.0 |
| `ctx serve` | MCP server (HTTP + stdio) |
| `ctx watch` | File watcher with debounce |
| `ctx push/pull/share/login` | [preview] Cloud sync — needs ctx backend; local-first otherwise |

## MCP integration (stdio-first)

**Recommended: stdio** — full MCP protocol (JSON-RPC), works with Cursor, Claude Desktop, and any MCP client:

```json
{"mcpServers": {"ctx": {"command": "ctx", "args": ["serve", "--stdio"]}}}
```

`ctx init` can write this for you: choose `cursor` (writes `.cursor/mcp.json`), `vscode` (writes `.vscode/mcp.json`), or `claude-desktop` (merges into the Claude Desktop user config, preserving your other servers).

**HTTP mode** (`ctx serve --port 3100`) is a plain REST API for scripts and debugging (`/context`, `/context/apis`, `/mcp/tools/*`) — it is *not* the MCP Streamable HTTP protocol, so point MCP clients at stdio.

Tools: `get_project_context` (supports `sections`, `max_tokens`), `get_context_for_task`, `get_context_for_file`, `get_api_endpoints`, `get_database_schema`, `get_project_conventions`, `get_env_requirements`, `search_context`.

Tip: `GET /context?max_tokens=4000` returns budget-truncated context (least-important sections dropped first) so large repos fit model windows.

## Supported stack matrix

| Area | Coverage |
|---|---|
| Languages | TypeScript/JavaScript, Go, Python, Rust, Java/Kotlin (detection); TS/JS, Go, Python (route extraction) |
| Frameworks | Next.js (App + Pages router), Express/Fastify/Hono, Gin/Echo/Fiber/Chi/Mux, Flask/FastAPI/Django |
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

Uses `sentence-transformers/all-MiniLM-L6-v2` when installed, TF-IDF fallback otherwise (Go and Python rankers are parity-tested). `ctx extract` auto-merges sidecar AST routes when the sidecar is reachable; offline it silently falls back to regex extractors.

## Privacy

Never stores `.env` values — only names, categories, required flags. Sensitive defaults redacted (`[REDACTED]`). `.ctxignore` supports gitignore-style patterns (`*.log`, `secrets/`, `**`, `!` negation).

## Config (`~/.config/ctx/config.toml` + `.ctx/config.toml`)

`core.api_url/ml_url`, `extraction.*` toggles, `privacy.*`, `sync.*`.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `not initialized (run ctx init)` | Run `ctx init` in the project root first |
| `ctx diff` → `no parent at HEAD~1` | Only one snapshot exists; run `ctx extract` after changing code |
| `search` returns nothing | Run `ctx extract` first; check `.ctx/context.json` exists |
| `extract` always says "No changes" | Correct — snapshots dedupe on identical content; edit code to get a new snapshot |
| MCP `serve` port in use | `ctx serve --port 3200` |
| Sidecar AST adds nothing | Start it (`uvicorn ctx-ml.app:app --port 8001`); Go falls back silently when down |
| `go: no required module` | Module is `github.com/AnshGajera/CTX`; run `go mod tidy` |

## Contributing / License

MIT. See `Makefile` (`build/test/lint/release`). Run `go test ./...` and `python -m pytest ctx-ml/tests` before PRs.
