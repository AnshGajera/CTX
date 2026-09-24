# CTX — Context Engine for Software Development

> Git for Context, not Code. Extract structured project context (architecture, APIs, DB schemas, deps, env, patterns) and serve it to AI tools via MCP. Hybrid TF-IDF + MiniLM retrieval for RAG. Local dashboard, health scoring, and a self-hosted team server included.

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
| `ctx watch` | File watcher with debounce (single-flight re-extract) |
| `ctx push/pull/share/login` | Team sync via `ctx-server` (self-hosted, JWT) |
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

**HTTP mode** (`ctx serve --port 3100`) is a plain REST API for scripts and debugging (`/context`, `/context/apis`, `/mcp/tools/*`) — it is *not* the MCP Streamable HTTP protocol, so point MCP clients at stdio. Binds `127.0.0.1` by default (`--bind 0.0.0.0` only on trusted networks), with server timeouts and 5MB body caps.

Tools: `get_project_context` (supports `sections`, `max_tokens`), `get_context_for_task`, `get_context_for_file`, `get_api_endpoints`, `get_database_schema`, `get_project_conventions`, `get_env_requirements`, `search_context`.

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
# or: docker compose up ctx-server      # :3200 with /data volume
```

Point the CLI at it via `core.api_url`, then `ctx login` (`--token` skips the echoing password prompt; `CTX_TOKEN` env also works), `ctx push/pull`, `ctx share --create-token`. Orgs/projects/history/share/token endpoints live under `/api/v1/*` (60 req/min + burst 10, 10MiB/100-snapshot caps). Pulled snapshots are hash-verified before touching disk.

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
| `ctx login` password echoes | Expected (no masking) — prefer `ctx login --token` or `CTX_TOKEN` |
| Sidecar AST adds nothing | Start it (`uvicorn ctx-ml.app:app --port 8001`); Go falls back silently when down |
| `go: no required module` | Module is `github.com/AnshGajera/CTX`; run `go mod tidy` |

## Contributing / License

MIT. See `Makefile` (`build/test/test-go/test-python/lint/release`). Run `go test ./...` and `python -m pytest ctx-ml/tests` before PRs. See `CONTRIBUTING.md` and `SECURITY.md`.

Version: `0.1.5` — bump with `ctx bump patch` or `make build VERSION=0.2.0`.
