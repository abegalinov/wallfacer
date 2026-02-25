# Architecture

Wallfacer is a Kanban task runner that executes Claude Code in isolated sandbox containers. Users create tasks on a web board; dragging a card from Backlog to In Progress triggers autonomous AI execution in an isolated git worktree, with auto-merge back to the main branch on completion.

## System Overview

```
┌─────────────────────────────────────────────────────────────┐
│  Browser (Vanilla JS + Tailwind + Sortable.js)              │
│  5-column Kanban board — SSE for live updates               │
└────────────────────────┬────────────────────────────────────┘
                         │ HTTP / SSE
┌────────────────────────▼────────────────────────────────────┐
│  Go Server (native on host)                                 │
│  main.go · handler.go · runner.go · store.go · git.go      │
└──────┬──────────────────────────────────────┬───────────────┘
       │ os/exec (podman/docker)              │ git commands
┌──────▼──────────────┐              ┌────────▼──────────────┐
│  Sandbox Container  │              │  Git Worktrees        │
│  Ubuntu 24.04       │              │  ~/.wallfacer/        │
│  Claude Code CLI    │◄────mount────│  worktrees/<uuid>/    │
└─────────────────────┘              └───────────────────────┘
```

The Go server runs natively on the host and persists tasks to per-task directories. It launches ephemeral sandbox containers via `podman run` (or `docker run`). Each task gets its own git worktree so multiple tasks can run concurrently without interfering.

## Technology Stack

**Backend** — Go 1.25, stdlib `net/http` (no framework), `os/exec` for containers, `sync.RWMutex` for concurrency, `github.com/google/uuid` for task IDs.

**Frontend** — Vanilla JavaScript, Tailwind CSS, Sortable.js, Marked.js. `EventSource` (SSE) for live updates, `localStorage` for theme preferences.

**Infrastructure** — Podman or Docker as container runtime. Ubuntu 24.04 sandbox image with Claude Code CLI installed. Git worktrees for per-task isolation.

**Persistence** — Filesystem only, no database. `~/.wallfacer/data/<uuid>/` per task. Atomic writes via temp file + `os.Rename`.

## Project Structure

```
wallfacer/
├── main.go              # CLI dispatch, container runtime detection, server init, browser launch
├── server.go            # HTTP server setup, mux construction, route registration
│
├── internal/
│   ├── envconfig/       # .env file parsing and atomic update helpers
│   ├── gitutil/         # Git operations: repo queries, worktree lifecycle, rebase/merge, status
│   ├── handler/         # HTTP API handlers (one file per concern)
│   │   ├── config.go        # GET /api/config
│   │   ├── containers.go    # GET /api/containers
│   │   ├── env.go           # GET/PUT /api/env
│   │   ├── execute.go       # Task lifecycle actions (feedback, done, cancel, resume, sync, archive)
│   │   ├── git.go           # Git status, push, sync, branches, checkout, create-branch, diff
│   │   ├── instructions.go  # GET/PUT /api/instructions, POST reinit
│   │   ├── stream.go        # SSE endpoints (task stream, git stream, container logs)
│   │   └── tasks.go         # Task CRUD, title generation
│   ├── instructions/    # Workspace CLAUDE.md management
│   ├── logger/          # Structured logging (pretty-print + JSON)
│   ├── runner/          # Container orchestration, task execution, commit pipeline
│   │   ├── board.go         # Board context (board.json) generation for cross-task awareness
│   │   ├── commit.go        # Commit pipeline: Claude commit, rebase, merge, cleanup
│   │   ├── container.go     # Container argument building, execution, output parsing
│   │   ├── execute.go       # Main task execution loop, worktree sync
│   │   ├── runner.go        # Runner struct, config, container listing (Podman + Docker)
│   │   ├── snapshot.go      # Pre-run workspace snapshot for diff baselines
│   │   ├── title.go         # Background title generation via Claude
│   │   └── worktree.go      # Worktree setup and cleanup
│   └── store/           # Per-task directory persistence, data models, event sourcing
│
├── ui/
│   ├── index.html       # 5-column Kanban board layout
│   ├── css/
│   │   ├── styles.css       # Custom component styles
│   │   └── tailwind.css     # Tailwind CSS build
│   └── js/
│       ├── state.js         # Global state management
│       ├── api.js           # HTTP client & SSE stream setup
│       ├── tasks.js         # Task CRUD operations
│       ├── render.js        # Board rendering & DOM updates
│       ├── modal.js         # Task detail modal (diff view, events, logs)
│       ├── git.js           # Git status display & branch switcher
│       ├── dnd.js           # Drag-and-drop (Sortable.js)
│       ├── events.js        # Event timeline rendering
│       ├── envconfig.js     # API configuration editor (token, base URL, model)
│       ├── containers.js    # Container monitoring UI
│       ├── instructions.js  # CLAUDE.md editor
│       ├── markdown.js      # Markdown rendering (Marked.js)
│       ├── theme.js         # Dark/light theme toggle
│       └── utils.js         # Shared utility functions
│
├── sandbox/
│   ├── Dockerfile       # Ubuntu 24.04 + Go + Node + Python + Claude Code
│   └── entrypoint.sh    # Git config setup, Claude Code launcher
│
├── Makefile             # build, server, run, shell, clean targets
├── go.mod, go.sum
└── docs/                # Documentation
```

## Design Choices

| Choice | Rationale |
|---|---|
| Git worktrees per task | Full isolation; concurrent tasks don't interfere; Claude sees a clean branch |
| Goroutines, no queue | Simplicity; Go's scheduler handles parallelism; tasks are long-running and IO-bound |
| Filesystem persistence, no DB | Zero dependencies; atomic rename is crash-safe; human-readable for debugging |
| SSE, not WebSocket | Simpler server-side; one-directional push is all the UI needs |
| Ephemeral containers | No state leaks between tasks; each run starts clean |
| Event sourcing (traces/) | Full audit trail; enables crash recovery and replay |
| Board context (`board.json`) | Cross-task awareness; Claude can see sibling tasks to avoid conflicts |
| Auto-detect container runtime | Supports both Podman and Docker transparently |

## Configuration

### CLI Subcommands

- `wallfacer run [flags] [workspace ...]` — Start the Kanban server
- `wallfacer env` — Show configuration and env file status

Running `wallfacer` with no arguments prints help.

### Flags for `wallfacer run`

All flags have env var fallbacks:

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `-addr` | `ADDR` | `:8080` | Listen address |
| `-data` | `DATA_DIR` | `~/.wallfacer/data` | Data directory |
| `-container` | `CONTAINER_CMD` | auto-detected | Container runtime command (podman or docker) |
| `-image` | `SANDBOX_IMAGE` | `wallfacer:latest` | Sandbox container image |
| `-env-file` | `ENV_FILE` | `~/.wallfacer/.env` | Env file passed to containers |
| `-no-browser` | — | `false` | Do not open browser on start |

Positional arguments after flags are workspace directories to mount (defaults to current directory).

The `-container` flag defaults to auto-detection: it checks `/opt/podman/bin/podman` first, then `podman` on `$PATH`, then `docker` on `$PATH`. Override with `CONTAINER_CMD` env var or `-container` flag to use a specific runtime.

### Environment File

`~/.wallfacer/.env` is passed into every sandbox container via `--env-file`. The server also parses it to extract the model override.

At least one authentication variable must be set:

| Variable | Required | Description |
|---|---|---|
| `CLAUDE_CODE_OAUTH_TOKEN` | one of these two | OAuth token from `claude setup-token` (Claude Pro/Max) |
| `ANTHROPIC_API_KEY` | one of these two | Direct API key from console.anthropic.com |
| `ANTHROPIC_BASE_URL` | no | Custom API endpoint; defaults to `https://api.anthropic.com` |
| `CLAUDE_CODE_MODEL` | no | Model passed as `--model` to every `claude` invocation; omit to use the Claude Code default |

When both `CLAUDE_CODE_OAUTH_TOKEN` and `ANTHROPIC_API_KEY` are set, the OAuth token takes precedence. This is Claude Code CLI behavior — wallfacer simply passes both variables through to the container via `--env-file`.

All four variables can be edited at runtime from **Settings → API Configuration** in the web UI. Changes take effect on the next task run without restarting the server.

`wallfacer env` reports the status of all four variables.

## Server Initialization

`main.go` → `runServer`:

```
parse CLI flags / env vars
→ load tasks from data/<uuid>/task.json into memory
→ create worktreesDir (~/.wallfacer/worktrees/)
→ pruneOrphanedWorktrees()   (removes stale worktree dirs + runs `git worktree prune`)
→ recover crashed tasks      (in_progress / committing → failed)
→ register HTTP routes
→ start listener on :8080
→ open browser (unless -no-browser)
```
