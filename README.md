# Wallfacer

A Kanban task runner for [Claude Code](https://claude.ai/code). Create tasks as cards in a web UI, drag them to "In Progress" to trigger Claude Code execution in a sandbox container, and inspect results when done.

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [Podman](https://podman.io/) or [Docker](https://www.docker.com/)
- Claude Pro or Max subscription (for the OAuth token)

## Quick Start

**1. Get an OAuth token**

```bash
claude setup-token
```

**2. Configure the environment file**

```bash
mkdir -p ~/.wallfacer
cp .env.example ~/.wallfacer/.env
# Edit ~/.wallfacer/.env and paste your token
```

**3. Build the sandbox image**

```bash
make build
```

**4. Build and start the server**

```bash
go build -o wallfacer .
wallfacer run ~/projects/myapp ~/projects/mylib
```

The browser opens automatically to http://localhost:8080.

## Usage

```bash
# Mount specific workspace directories
wallfacer run ~/project1 ~/project2

# Defaults to current directory if no args given
wallfacer run

# Custom port, skip auto-opening the browser
wallfacer run -addr :9090 -no-browser ~/myapp

# Show configuration and env file status
wallfacer env

# All flags
wallfacer run -help
```

### How It Works

1. Create a task card in the Backlog column with a prompt for Claude
2. Drag it to In Progress — Wallfacer launches a sandbox container and runs Claude Code
3. When Claude finishes, the card moves to Done and changes are committed to your repo
4. If Claude asks a question, the card moves to Waiting — submit feedback to continue

See [Task Lifecycle](docs/task-lifecycle.md) for details on all states and transitions.

### Make Targets

| Target | Description |
|---|---|
| `make build` | Build the sandbox image |
| `make server` | Build and run the Go server |
| `make run PROMPT="..."` | Headless one-shot Claude execution |
| `make shell` | Debug shell inside a sandbox container |
| `make clean` | Remove the sandbox image |

## Documentation

- [Architecture](docs/architecture.md) — system overview, tech stack, project structure, configuration
- [Task Lifecycle](docs/task-lifecycle.md) — states, turn loop, feedback, data models, persistence
- [Git Worktrees](docs/git-worktrees.md) — per-task isolation, commit pipeline, conflict resolution
- [Orchestration](docs/orchestration.md) — API routes, container execution, SSE, concurrency

## Origin Story

It started innocently enough. A developer, keyboard in hand, neurons firing, writing actual code like some kind of 2023 caveman. Line by line. Bracket by bracket. The usual suffering.

Then Claude Code arrived. Suddenly the developer was mostly writing *task descriptions* instead of code. The ratio of English words to Go syntax in daily output shifted dramatically. Productivity soared. Understanding of what was actually happening in the codebase plummeted at roughly the same rate. A fair trade.

The project grew. A Go server. A Kanban board. A sandbox container. A whole little world for running Claude Code tasks. And somewhere around the point where dragging a card from Backlog to In Progress actually worked, a horrifying realization set in:

*The tool was ready to use.*

So the developer opened Wallfacer, created a task card that said "add retry logic to failed tasks," dragged it to In Progress, and watched Claude Code — running inside a Wallfacer sandbox — implement a feature for Wallfacer.

The commits started coming from inside the house.

## License

See [LICENSE](LICENSE).
