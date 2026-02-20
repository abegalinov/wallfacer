# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Wallfacer is a Kanban task runner for Claude Code. It provides a web UI where tasks are created as cards, dragged to "In Progress" to trigger Claude Code execution in an isolated sandbox container, and results are inspected when done.

**Architecture:** Browser → Go server (:8080) → per-task directory storage (`data/<uuid>/`). The server runs natively on the host and launches ephemeral sandbox containers via `os/exec` (podman/docker). Each task gets its own git worktree for isolation.

For detailed documentation see `docs/`.

## Build & Run Commands

```bash
make build          # Build the wallfacer sandbox image
make server         # Build and run the Go server natively
make shell          # Open bash shell in sandbox container for debugging
make clean          # Remove the sandbox image
make run PROMPT="…" # Headless one-shot Claude execution with a prompt
```

CLI usage (after `go build -o wallfacer .`):

```bash
wallfacer                                    # Print help
wallfacer run ~/project1 ~/project2          # Mount workspaces, open browser
wallfacer run                                # Defaults to current directory
wallfacer run -addr :9090 -no-browser        # Custom port, no browser
wallfacer env                                # Show config and env status
```

The Makefile uses Podman (`/opt/podman/bin/podman`) by default. Adjust `PODMAN` variable if using Docker.

## Server Development

The Go source lives at the top level. Module path: `changkun.de/wallfacer`. Go version: 1.25.7.

```bash
go build -o wallfacer .   # Build server binary
go vet ./...              # Lint
```

There are no tests currently. The server uses `net/http` stdlib routing (Go 1.22+ pattern syntax) with no framework.

Key server files:
- `main.go` — Subcommand dispatch, CLI flags, workspace resolution, HTTP routing, browser launch
- `handler.go` — API handlers: tasks CRUD, feedback, resume, complete, SSE streaming
- `runner.go` — Container orchestration via `os/exec`; task execution loop; commit pipeline; usage tracking
- `store.go` — Per-task directory persistence, data models (Task, TaskUsage, TaskEvent), event sourcing
- `git.go` — Git worktree operations, branch detection, rebase/merge
- `logger.go` — Structured logging
- `ui/index.html` + `ui/js/` — Kanban board UI (vanilla JS + Tailwind CSS CDN + Sortable.js)

## API Routes

See `docs/orchestration.md` for full details.

- `GET /` — Kanban UI
- `GET /api/tasks` — List all tasks
- `POST /api/tasks` — Create task (JSON: `{prompt, timeout}`)
- `PATCH /api/tasks/{id}` — Update status/position/prompt/timeout
- `DELETE /api/tasks/{id}` — Delete task
- `POST /api/tasks/{id}/feedback` — Submit feedback for waiting tasks
- `POST /api/tasks/{id}/done` — Mark waiting task as done (triggers commit-and-push)
- `POST /api/tasks/{id}/resume` — Resume failed task with existing session
- `POST /api/tasks/{id}/archive` — Move done task to archived
- `POST /api/tasks/{id}/unarchive` — Restore archived task
- `GET /api/tasks/stream` — SSE: push task list on state change
- `GET /api/tasks/{id}/events` — Task event timeline
- `GET /api/tasks/{id}/outputs/{filename}` — Raw Claude Code output per turn
- `GET /api/tasks/{id}/logs` — SSE: stream live container logs
- `GET /api/git/status` — Git status for all workspaces
- `GET /api/git/stream` — SSE: git status updates
- `POST /api/git/push` — Push a workspace

## Task Lifecycle

States: `backlog` → `in_progress` → `done` | `waiting` | `failed` | `archived`

See `docs/task-lifecycle.md` for the full state machine, turn loop, and data models.

- Drag Backlog → In Progress triggers `runner.Run()` in a background goroutine
- Claude `end_turn` → commit pipeline → Done
- Empty stop_reason → Waiting (needs user feedback)
- `max_tokens`/`pause_turn` → auto-continue in same session
- Feedback on Waiting → resumes execution
- "Mark as Done" on Waiting → Done + auto commit-and-push
- "Resume" on Failed → continues in existing session
- "Retry" on Failed/Done → resets to Backlog with fresh session

## Key Conventions

- **UUIDs** for all task IDs (auto-generated via `github.com/google/uuid`)
- **Event sourcing** via per-task trace files; types: `state_change`, `output`, `feedback`, `error`
- **Per-task directory storage** with atomic writes (temp file + rename); `sync.RWMutex` for concurrency
- **Git worktrees** per task for isolation; see `docs/git-worktrees.md`
- **Usage tracking** accumulates input/output tokens, cache tokens, and cost across turns
- **Container execution** creates ephemeral containers via `os/exec`; mounts worktrees under `/workspace/<basename>`
- **Frontend** uses SSE for live updates; escapes HTML to prevent XSS
- **No framework** on backend (stdlib `net/http`) or frontend (vanilla JS)

## Configuration

See `docs/architecture.md#configuration` for the full reference.

Required: `~/.wallfacer/.env` with `CLAUDE_CODE_OAUTH_TOKEN` (from `claude setup-token`).
