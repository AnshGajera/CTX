# CTX — Context Engine for Software Development

> Git for Context, not Code. Extract structured project context (architecture, APIs, DB schemas, deps, env, patterns) and serve it to AI tools via MCP. Hybrid TF-IDF + MiniLM retrieval for RAG.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/ctxdev/ctx/main/scripts/install.sh | bash
# or
go build -o ctx ./cmd/ctx
```

## Quickstart

```bash
ctx init
ctx extract
ctx status
ctx search "auth flow"
ctx serve --port 3100        # HTTP MCP
ctx serve --stdio            # Cursor / Claude Desktop
```

## Commands

| Command | Description |
|---|---|
| `ctx init [--name]` | Detect stack, write `.ctx/`, install git hook |
| `ctx extract` | Run extractors, sanitize, save + snapshot |
| `ctx status` | Counts, branch, diff vs parent |
| `ctx diff [h1] [h2]` | Colored snapshot diff (supports `HEAD~N`) |
| `ctx log` | Snapshot history |
| `ctx search <q>` | Hybrid semantic search over context |
| `ctx eval` | Retrieval `hit@k` self-eval |
| `ctx serve` | MCP server (HTTP + stdio) |
| `ctx watch` | File watcher with debounce |
| `ctx push/pull/share/login` | Cloud sync (needs backend; local-first otherwise) |

## MCP integration

Cursor: add `http://localhost:3100/mcp` as MCP server.
Claude Desktop (`claude_desktop_config.json`):

```json
{"mcpServers": {"ctx": {"command": "ctx", "args": ["serve", "--stdio"]}}}
```

Tools: `get_project_context`, `get_context_for_task`, `get_context_for_file`, `get_api_endpoints`, `get_database_schema`, `get_project_conventions`, `get_env_requirements`.

## ML sidecar (`ctx-ml/`)

```bash
pip install -r ctx-ml/requirements.txt
uvicorn ctx-ml.app:app --port 8001
# or: docker compose up ctx-ml
python ctx-ml/eval.py .ctx/context.json 5
```

Uses `sentence-transformers/all-MiniLM-L6-v2` when available, TF-IDF fallback otherwise (Go and Python rankers are parity-tested).

## Privacy

Never stores `.env` values — only names, categories, required flags. Sensitive defaults redacted (`[REDACTED]`). Respect `.ctxignore`.

## Config (`~/.config/ctx/config.toml` + `.ctx/config.toml`)

`core.api_url/ml_url`, `extraction.*` toggles, `privacy.*`, `sync.*`.

## Contributing / License

MIT. See `Makefile` (`build/test/lint/release`).
