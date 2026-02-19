# Wallfacer

A Kanban task runner for Claude Code. Create tasks as cards in a web UI, drag them to In Progress to trigger Claude Code execution in a sandbox, and inspect results when done.

## Architecture

```
Browser (Kanban UI)
    │
    ↓
Go Server (:8080)  ──JSON file──→  data.json
    │
    ↓ (os/exec → podman run)
Sandbox Container (ephemeral) → Claude Code CLI
```

The Go server runs natively on the host and persists tasks to a local JSON file. It launches ephemeral sandbox containers via `podman run` (or `docker run`).

## Setup

```bash
# 1. Get an OAuth token (needs a browser)
claude setup-token

# 2. Configure
cp .env.example .env
# Edit .env and paste your token

# 3. Build sandbox image
make build

# 4. Start the server
make server
```

Open http://localhost:8080 to use the Kanban board.

## Task Lifecycle

```
BACKLOG ──drag──→ IN_PROGRESS ──auto──→ DONE
                      │
                      ├──auto──→ WAITING ──feedback──→ IN_PROGRESS
                      │
                      └──auto──→ FAILED
```

- Drag a card from Backlog to In Progress to start execution
- Claude finishes (`end_turn`) → card moves to Done
- Claude asks a question (empty stop_reason) → card moves to Waiting
- Submit feedback on a Waiting card → resumes execution
- `max_tokens` / `pause_turn` → auto-continues in the background

## Make Targets

| Target | Description |
|---|---|
| `make build` | Build the sandbox image |
| `make server` | Build and run the Go server natively |
| `make run PROMPT="…"` | Headless one-shot Claude execution |
| `make shell` | Open a bash shell in a sandbox container |
| `make clean` | Remove the sandbox image |

## Project Structure

```
.
├── Makefile                  # Top-level convenience targets
├── main.go               # Config, store init, HTTP routes, server startup
├── handler.go            # API handlers: tasks CRUD, feedback, events
├── runner.go             # Container orchestration via os/exec
├── store.go              # JSON file-backed persistence
├── ui/
│   └── index.html        # Kanban board UI
├── go.mod
├── go.sum
├── sandbox/
│   ├── Dockerfile            # Ubuntu 24.04 + Go + Node + Python + Claude Code
│   ├── entrypoint.sh         # Git safe.directory fix, launches Claude
│   └── .dockerignore
└── traces/            # Execution traces
```

## Configuration

Set these in `.env`:

| Variable | Description |
|---|---|
| `CLAUDE_CODE_OAUTH_TOKEN` | OAuth token from `claude setup-token` |

Server environment variables:

| Variable | Default | Description |
|---|---|---|
| `ADDR` | `:8080` | Listen address |
| `DATA_FILE` | `data.json` | Path to JSON persistence file |
| `CONTAINER_CMD` | `/opt/podman/bin/podman` | Path to container runtime binary |
| `SANDBOX_IMAGE` | `wallfacer:latest` | Sandbox container image |
| `ENV_FILE` | (empty) | Path to env file passed to sandbox |
| `WORKSPACES` | (empty) | Space-separated host paths to mount |

## Requirements

- [Go](https://go.dev/) 1.25+
- [Podman](https://podman.io/) (or Docker)
- Claude Pro or Max subscription (for OAuth token)

## License

See [LICENSE](LICENSE).
