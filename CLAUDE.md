# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Wallfacer is a Kanban task runner for Claude Code. It provides a web UI where tasks are created as cards, dragged to "In Progress" to trigger Claude Code execution in an isolated sandbox container, and results are inspected when done.

**Architecture:** Browser → Go server (:8080) → per-task directory storage (`data/<uuid>/`). The server runs natively on the host and launches ephemeral sandbox containers via `os/exec` (podman/docker).

## Build & Run Commands

```bash
make build          # Build the wallfacer sandbox image
make server         # Build and run the Go server natively
make shell          # Open bash shell in sandbox container for debugging
make clean          # Remove the sandbox image
make run PROMPT="…" # Headless one-shot Claude execution with a prompt
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
- `main.go` — Config via env vars, store init, HTTP routing, server startup
- `handler.go` — API handlers: tasks CRUD, feedback submission, event retrieval
- `runner.go` — Container orchestration via `os/exec`; creates/runs/parses sandbox output
- `store.go` — Per-task directory persistence (`data/<uuid>/task.json` + `traces/`), data models (Task, TaskEvent)
- `ui/index.html` — Kanban board UI (vanilla JS + Tailwind CSS CDN + Sortable.js)

## API Routes

- `GET /` — Kanban UI (embedded UI files)
- `GET /api/tasks` — List all tasks
- `POST /api/tasks` — Create task (JSON: `{prompt}`)
- `PATCH /api/tasks/{id}` — Update status/position
- `DELETE /api/tasks/{id}` — Delete task
- `POST /api/tasks/{id}/feedback` — Submit feedback for waiting tasks
- `GET /api/tasks/{id}/events` — Task event timeline

## Task Lifecycle

States: `backlog` → `in_progress` → `done` | `waiting` | `failed`

- Drag Backlog → In Progress triggers `runner.Run()` in a background goroutine
- Claude `end_turn` → Done; empty stop_reason → Waiting (needs user feedback)
- `max_tokens`/`pause_turn` → auto-continue in same session
- Feedback on Waiting card resumes execution

## Key Conventions

- **UUIDs** for all task IDs (auto-generated via `github.com/google/uuid`)
- **Event sourcing** via per-task trace files (`data/<uuid>/traces/0001.json`, ...); types: `state_change`, `output`, `feedback`, `error`
- **Per-task directory storage** (`data/<uuid>/task.json` for metadata, `traces/` for events); atomic writes (temp file + rename); `sync.RWMutex` for concurrency
- **Container execution** creates sibling containers via `os/exec`; mounts host workspaces under `/workspace/<basename>`
- **Frontend** polls every 2 seconds; uses optimistic UI updates; escapes HTML to prevent XSS
- **No framework** on backend (stdlib `net/http`) or frontend (vanilla JS)

## Environment

Server env vars: `ADDR`, `DATA_DIR`, `CONTAINER_CMD`, `SANDBOX_IMAGE`, `ENV_FILE`, `WORKSPACES`

Sandbox env (in `.env`): `CLAUDE_CODE_OAUTH_TOKEN` (required)
