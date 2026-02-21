package runner

import (
	"os/exec"
	"strings"
	"time"

	"changkun.de/wallfacer/internal/store"
	"github.com/google/uuid"
)

const (
	maxRebaseRetries   = 3
	defaultTaskTimeout = 15 * time.Minute
)

// RunnerConfig holds all configuration needed to construct a Runner.
type RunnerConfig struct {
	Command          string
	SandboxImage     string
	EnvFile          string
	Workspaces       string // space-separated workspace paths
	WorktreesDir     string
	InstructionsPath string
}

// Runner orchestrates Claude Code container execution for tasks.
// It manages worktree isolation, container lifecycle, and the commit pipeline.
type Runner struct {
	store            *store.Store
	command          string
	sandboxImage     string
	envFile          string
	workspaces       string
	worktreesDir     string
	instructionsPath string
}

// NewRunner constructs a Runner from the given store and config.
func NewRunner(s *store.Store, cfg RunnerConfig) *Runner {
	return &Runner{
		store:            s,
		command:          cfg.Command,
		sandboxImage:     cfg.SandboxImage,
		envFile:          cfg.EnvFile,
		workspaces:       cfg.Workspaces,
		worktreesDir:     cfg.WorktreesDir,
		instructionsPath: cfg.InstructionsPath,
	}
}

// Command returns the container runtime binary path (podman/docker).
func (r *Runner) Command() string {
	return r.command
}

// Workspaces returns the list of configured workspace paths.
func (r *Runner) Workspaces() []string {
	if r.workspaces == "" {
		return nil
	}
	return strings.Fields(r.workspaces)
}

// KillContainer sends a kill signal to the running container for a task.
// Safe to call when no container is running — errors are silently ignored.
func (r *Runner) KillContainer(taskID uuid.UUID) {
	containerName := "wallfacer-" + taskID.String()
	exec.Command(r.command, "kill", containerName).Run()
}
