# Contributing to CTX

Thank you for your interest in contributing to CTX! This guide will help you get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Commit Conventions](#commit-conventions)
- [Pull Request Process](#pull-request-process)
- [Adding Extractors](#adding-extractors)

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Getting Started

1. **Fork** the repository on GitHub
2. **Clone** your fork locally
3. **Create a branch** for your feature or fix
4. **Make changes**, write tests, ensure they pass
5. **Submit** a pull request

## Development Setup

### Prerequisites

- **Go 1.24+** — [install](https://go.dev/dl/)
- **Python 3.11+** — for `ctx-ml` sidecar (optional)
- **Git** — for version control

### Build & Test

```bash
# Clone
git clone https://github.com/<you>/CTX.git
cd CTX

# Build
go build -o ctx ./cmd/ctx

# Run all Go tests
go test ./... -race

# Run Python tests (optional, for ctx-ml changes)
pip install -r ctx-ml/requirements.txt
python -m pytest ctx-ml/tests -q

# Lint
go vet ./...
# or with golangci-lint installed:
golangci-lint run ./...
```

### Quick Dev Loop

```bash
go run ./cmd/ctx init
go run ./cmd/ctx extract
go run ./cmd/ctx status
go run ./cmd/ctx search "auth flow"
```

## Making Changes

### Project Structure

```
cmd/ctx/          — CLI entry point
internal/
  ai/             — TF-IDF ranking, chunking
  cache/          — File tree state caching
  cli/            — CLI app + commands
  config/         — TOML configuration
  context/        — Data models (ProjectContext, etc.)
  detector/       — Language/framework detection
  engine/         — Extraction orchestration
  extractors/     — Individual extractors (one per file)
  mcp/            — MCP server (HTTP + stdio)
  openapi/        — OpenAPI export
  privacy/        — Secret redaction & sanitization
  setup/          — Editor integration (Cursor, VSCode, etc.)
  sync/           — Cloud sync client
  tui/            — Terminal UI (banners, prompts)
  version/        — Build version injection
  versioning/     — Snapshot store, diffs
  watcher/        — File watcher
ctx-ml/           — Python ML sidecar
server/           — Backend API server (in development)
```

### Code Style

- **Go**: Follow standard Go conventions (`gofmt`, `go vet`). Use `golangci-lint` for comprehensive linting.
- **Python**: Follow PEP 8. Use type hints.
- **Comments**: Preserve existing comments. Add doc comments for exported types and functions.
- **Error handling**: Always wrap errors with `fmt.Errorf("context: %w", err)`.
- **Tests**: Add tests for new functionality. Place them in `*_test.go` files next to the code.

## Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]
```

**Types:**
- `feat` — New feature
- `fix` — Bug fix
- `docs` — Documentation
- `test` — Adding or updating tests
- `refactor` — Code change that neither fixes a bug nor adds a feature
- `ci` — CI/CD changes
- `chore` — Maintenance tasks

**Examples:**
```
feat(extractors): add Ruby on Rails route extraction
fix(privacy): handle nested secret patterns in YAML
docs: add self-hosting guide
test(sync): add integration tests for push/pull
```

## Pull Request Process

1. **Update tests** — All PRs must include relevant tests
2. **Run checks locally** — `go test ./... -race && go vet ./...`
3. **One concern per PR** — Keep PRs focused and reviewable
4. **Describe the change** — Use the PR template; explain *why*, not just *what*
5. **Screenshots for UI changes** — If you change TUI output, include before/after
6. **Breaking changes** — Clearly document in the PR description

## Adding Extractors

CTX's value grows with every new extractor. Here's how to add one:

### 1. Implement the `Extractor` interface

```go
// internal/extractors/your_extractor.go
package extractors

type YourExtractor struct {
    root string
}

func NewYourExtractor(root string) *YourExtractor {
    return &YourExtractor{root: root}
}

func (e *YourExtractor) Name() string { return "your_extractor" }

func (e *YourExtractor) Extract(ctx *projctx.ProjectContext) error {
    // Populate relevant fields on ctx
    return nil
}
```

### 2. Register in the engine

Add your extractor to [`internal/engine/engine.go`](internal/engine/engine.go) under the appropriate section gate.

### 3. Add tests

Create `internal/extractors/your_extractor_test.go` with table-driven tests covering edge cases.

### 4. Update the README

Add your language/framework to the "Supported stack matrix" table.

---

## Questions?

Open a [Discussion](https://github.com/AnshGajera/CTX/discussions) or reach out in an issue. We're happy to help!
