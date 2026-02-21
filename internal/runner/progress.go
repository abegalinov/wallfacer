package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"changkun.de/wallfacer/internal/gitutil"
	"changkun.de/wallfacer/internal/logger"
	"changkun.de/wallfacer/internal/store"
)

// writeProgressMD appends a structured entry to PROGRESS.md in each workspace
// root (the main working tree, not the task worktree), then commits it.
func (r *Runner) writeProgressMD(task *store.Task, commitHashes map[string]string) error {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	result := "(no result recorded)"
	if task.Result != nil && *task.Result != "" {
		result = truncate(*task.Result, 1000)
	}

	for _, ws := range r.Workspaces() {
		hash := commitHashes[ws]
		if hash == "" {
			hash = "(no commit)"
		}

		entry := fmt.Sprintf(
			"\n## Task: %s\n\n**Date**: %s  \n**Branch**: %s  \n**Commit**: `%s`\n\n**Prompt**:\n> %s\n\n**Result**:\n%s\n\n---\n",
			task.ID.String()[:8],
			timestamp,
			task.BranchName,
			hash,
			strings.ReplaceAll(task.Prompt, "\n", "\n> "),
			result,
		)

		progressPath := filepath.Join(ws, "PROGRESS.md")

		// Ensure the file starts with a header if it doesn't exist yet.
		if _, err := os.Stat(progressPath); os.IsNotExist(err) {
			header := "# Progress Log\n\nRecords of completed tasks, problems encountered, and lessons learned.\n"
			if err := os.WriteFile(progressPath, []byte(header), 0644); err != nil {
				logger.Runner.Warn("create PROGRESS.md", "path", progressPath, "error", err)
				continue
			}
		}

		f, err := os.OpenFile(progressPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			logger.Runner.Warn("open PROGRESS.md", "path", progressPath, "error", err)
			continue
		}
		_, writeErr := f.WriteString(entry)
		f.Close()
		if writeErr != nil {
			logger.Runner.Warn("write PROGRESS.md", "path", progressPath, "error", writeErr)
			continue
		}

		if gitutil.IsGitRepo(ws) {
			exec.Command("git", "-C", ws, "add", "PROGRESS.md").Run()
			exec.Command("git", "-C", ws, "commit", "-m",
				fmt.Sprintf("wallfacer: progress log for task %s", task.ID.String()[:8]),
			).Run()
		}
	}
	return nil
}
