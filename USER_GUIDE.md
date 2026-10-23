# CTX User Guide

CTX is a local context engine for software projects. It scans a project,
extracts useful technical information, and makes that information available to
the CTX CLI, dashboard, and AI assistants through MCP.

This guide is for someone who has the CTX GitHub repository and wants to use
CTX on another project.

## What you need

For the normal workflow, you need:

- Git
- A project that you want CTX to analyze
- PowerShell on Windows, or Bash on Linux/macOS

You do not need Python, Docker, an AI API key, or a cloud account for the
basic CTX workflow.

## 1. Install CTX

### Windows: install the latest release

Open PowerShell and run:

```powershell
git clone https://github.com/AnshGajera/CTX.git
Set-Location .\CTX
powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1
```

Close and reopen PowerShell so the new `PATH` is loaded. Verify the
installation:

```powershell
ctx --help
ctx bump current
```

Confirm that the installed command includes the newer initialization options:

```powershell
ctx init --help
```

The help output should include `--yes`, `--editor`, and `--sections`. If
`--editor` is missing, your installed executable is an older CTX release.
Build the current repository version as described in [Build from source](#build-from-source),
or install a newer GitHub release when one is available.

The installer downloads a Windows release, verifies its checksum, and places
the executable in:

```text
%USERPROFILE%\.ctx\bin\ctx.exe
```

If `ctx` is not recognized, run it directly:

```powershell
& "$HOME\.ctx\bin\ctx.exe" --help
```

### Linux or macOS: install the latest release

```bash
git clone https://github.com/AnshGajera/CTX.git
cd CTX
bash ./scripts/install.sh
```

Restart the shell, then verify:

```bash
ctx --help
ctx bump current
```

### Build from source

Use this only when a suitable release is unavailable or you want to develop
CTX itself. Install Go 1.24 or newer, then run:

```powershell
git clone https://github.com/AnshGajera/CTX.git
Set-Location .\CTX
go mod download
go build -o .\ctx.exe .\cmd\ctx
.\ctx.exe --help
```

On Linux/macOS:

```bash
git clone https://github.com/AnshGajera/CTX.git
cd CTX
go mod download
go build -o ./ctx ./cmd/ctx
./ctx --help
```

## 2. Select the project CTX should analyze

CTX analyzes the directory in which its commands are run. It should normally
be run from the root of your own project, not from the CTX source repository.

Example:

```powershell
git clone https://github.com/example/my-project.git
Set-Location .\my-project
Get-ChildItem
```

The project root usually contains files such as `package.json`, `go.mod`,
`requirements.txt`, `Cargo.toml`, or a `src` directory.

## 3. Initialize CTX

For a quick setup with no AI-editor configuration:

```powershell
ctx init --yes --editor none
```

For an interactive setup:

```powershell
ctx init
```

The wizard detects the project and lets you choose context sections.

If your executable supports the `--editor` option, configure an AI editor
directly:

```powershell
ctx init --editor vscode
ctx init --editor cursor
ctx init --editor claude-desktop
```

If your installed executable is older and does not support `--editor`, run
`ctx init` without flags and complete the interactive wizard, or update CTX
from source before using the commands above.

Initialization creates the local `.ctx` directory. It contains generated
context, configuration, snapshots, and context-branch information.

## 4. Extract project context

Run:

```powershell
ctx extract
```

CTX scans the project and extracts information such as:

- Architecture, languages, and frameworks
- REST and GraphQL endpoints
- Database models and migrations
- Dependencies and package managers
- Environment-variable requirements
- File structure and project conventions
- Git state, snapshots, and useful patterns

Run extraction again whenever the project changes:

```powershell
ctx extract
```

If nothing relevant changed, CTX may report that no new snapshot is needed.
That is expected.

## 5. Inspect and search the result

Check the generated context:

```powershell
ctx status
ctx health
```

Search it using natural language:

```powershell
ctx search "where is authentication handled"
ctx search "list GraphQL mutations"
ctx search "which environment variables are required"
ctx search "show the database models for orders"
```

Export detected APIs:

```powershell
ctx export --format openapi -o .\openapi.json
```

For scripts and CI, use JSON output:

```powershell
ctx status --json
ctx health --json
ctx search "authentication" --json
```

## 6. Use the dashboard

From the project root, start the local dashboard:

```powershell
ctx serve --ui
```

Open:

```text
http://127.0.0.1:3100/ui
```

The dashboard displays extracted architecture, APIs, database information,
environment requirements, search results, and context history.

Use another port if required:

```powershell
ctx serve --ui --port 3200
```

Stop the server with `Ctrl+C`.

## 7. Connect CTX to an AI assistant

CTX does not replace Copilot, Cursor, Claude, or another generative AI model.
It supplies those tools with accurate, project-specific context.

The recommended MCP transport is stdio:

```powershell
ctx serve --stdio
```

When configured, the AI editor starts this command automatically. A typical
MCP configuration is:

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

The `ctx init` wizard can create the configuration for VS Code, Cursor, or
Claude Desktop. After setup, ask the AI assistant questions such as:

- Explain this project's architecture.
- Where is authentication implemented?
- Which files implement the user registration API?
- What database tables are related to orders?
- What environment variables are required to run this project?
- Add a new endpoint following the existing project conventions.

Before asking the AI assistant to work on changed code, refresh the context:

```powershell
ctx extract
```

## 8. Search and AI backends

CTX works without an external model:

1. It extracts structured project information locally.
2. It splits that information into searchable chunks.
3. It uses built-in TF-IDF search by default.
4. It returns relevant context through the CLI, dashboard, or MCP.

An optional Python sidecar adds semantic search using
`sentence-transformers/all-MiniLM-L6-v2`. If the sidecar or model is
unavailable, CTX falls back to TF-IDF.

Therefore, the optional ML setup is not required for normal use.

## 9. Optional: enable the Python ML sidecar

Install Python 3.11 or newer, then from the CTX repository:

```powershell
python -m venv .venv
.\.venv\Scripts\Activate.ps1
python -m pip install --upgrade pip
python -m pip install -r .\ctx-ml\requirements.txt
```

Start the sidecar:

```powershell
python -m uvicorn ctx-ml.app:app --host 127.0.0.1 --port 8001
```

Verify it:

```powershell
Invoke-RestMethod http://127.0.0.1:8001/health
```

To install the optional embedding dependencies as well:

```powershell
python -m pip install -r .\ctx-ml\requirements-ml.txt
```

The first MiniLM use may download model files. The model runs locally after
it has been downloaded. Configure the project's CTX settings with:

```toml
[core]
ml_url = "http://127.0.0.1:8001"
```

If you do not configure `ml_url`, the built-in TF-IDF search remains available.

## 10. Optional: run the services with Docker

Docker Desktop can run the ML sidecar and self-hosted team server:

```powershell
$env:CTX_JWT_SECRET = [guid]::NewGuid().ToString("N")
docker compose up -d
docker compose ps
```

Check the services:

```powershell
Invoke-RestMethod http://127.0.0.1:8001/health
Invoke-RestMethod http://127.0.0.1:3200/api/v1/health
```

Stop them:

```powershell
docker compose down
```

Use the team server only when multiple users need shared context,
authentication, push, pull, or project history. A single user can ignore it.

## 11. Manage context history

CTX can version extracted context separately from source-code history:

```powershell
ctx branch feature-auth
ctx checkout feature-auth
ctx extract
ctx commit -m "authentication endpoints discovered"
ctx log
ctx checkout main
ctx merge feature-auth
ctx tag v1.0.0-context
```

Useful comparison commands:

```powershell
ctx diff
ctx diff HEAD~1 HEAD
```

This is useful when an AI assistant investigates a feature without changing
the main context immediately.

## 12. Recommended daily workflow

From the root of the project being developed:

```powershell
ctx extract
ctx status
ctx health
ctx search "the task I am working on"
```

Then open the project in your AI editor and ask questions. After source-code
changes, run `ctx extract` again before requesting another context-aware
answer.

For a quick five-minute workflow:

```powershell
ctx init --yes --editor vscode
ctx extract
ctx health
ctx search "authentication"
ctx serve --ui
```

## 13. Privacy and safe usage

CTX is designed for local operation and redacts or excludes common secret
files. It records environment-variable names and requirements, not intended
secret values.

Before committing `.ctx` to a public repository:

1. Inspect the generated files.
2. Confirm that no credentials, tokens, private keys, or sensitive source
   content are present.
3. Add `.ctx` to `.gitignore` if the context should remain local.

Remember that a connected hosted AI assistant may process the context you
allow it to receive. Check that provider's privacy settings.

## 14. Troubleshooting

### `ctx` is not recognized

Restart the terminal after installation. On Windows, verify:

```powershell
Test-Path "$HOME\.ctx\bin\ctx.exe"
& "$HOME\.ctx\bin\ctx.exe" --help
```

### `flag provided but not defined: -editor`

The `ctx.exe` on your `PATH` is older than the source version documented in
this guide. Check its command:

```powershell
Get-Command ctx | Format-List Source
ctx init --help
```

To build the current version from the CTX repository:

```powershell
Set-Location D:\Path\To\CTX
go mod download
go build -o "$HOME\.ctx\bin\ctx.exe" .\cmd\ctx
```

Open a new PowerShell window, then verify:

```powershell
ctx init --help
```

Alternatively, use the older executable's supported workflow:

```powershell
ctx init
```

The editor-specific flags will not be available until the executable is
updated.

### `not initialized (run ctx init)`

Change to the project root and run:

```powershell
ctx init --yes --editor none
```

### Search returns no results

Run extraction first:

```powershell
ctx extract
ctx search "your query"
```

### The ML sidecar is unavailable

This is safe. CTX falls back to TF-IDF. Start the sidecar only if semantic
search or Python AST extraction is needed.

### The dashboard port is already in use

Use another port:

```powershell
ctx serve --ui --port 3200
```

### MCP reports that `required` is not an array

If the AI client reports an error such as:

```text
Invalid schema for function 'ctx-get_project_context':
None is not of type 'array'
```

update or rebuild CTX, then restart the AI editor:

```powershell
Set-Location D:\Path\To\CTX
go mod download
go build -o "$HOME\.ctx\bin\ctx.exe" .\cmd\ctx
```

Open a new PowerShell window and verify:

```powershell
ctx init --help
```

Then reload VS Code with `Developer: Reload Window`. Current CTX versions
publish `"required": []` for tools with no required arguments instead of
`"required": null`, which is required by clients that validate JSON Schema
strictly.

### No new snapshot is created

CTX deduplicates unchanged context. Change the project or run extraction
after a meaningful source change.

## 15. Command reference

| Command | Purpose |
|---|---|
| `ctx init` | Initialize CTX in the current project |
| `ctx extract` | Extract or refresh project context |
| `ctx status` | Show context and snapshot status |
| `ctx health` | Show context quality and improvement tips |
| `ctx search "..."` | Search extracted context |
| `ctx export --format openapi` | Export detected APIs |
| `ctx serve --ui` | Start the local dashboard |
| `ctx serve --stdio` | Start the MCP server for AI clients |
| `ctx branch` | List or create context branches |
| `ctx checkout` | Switch context branches or snapshots |
| `ctx commit -m "..."` | Save a context checkpoint |
| `ctx merge` | Merge another context branch |
| `ctx tag` | Mark a context milestone |
| `ctx diff` | Compare context snapshots |
| `ctx log` | Show context history |

For development and repository-level testing, see
[CONTRIBUTING.md](./CONTRIBUTING.md). For security reporting, see
[SECURITY.md](./SECURITY.md).
