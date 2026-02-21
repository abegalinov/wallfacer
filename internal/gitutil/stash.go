package gitutil

import (
	"log/slog"
	"os/exec"
	"strings"
)

// StashIfDirty stashes uncommitted changes in worktreePath if the working tree
// is dirty. Returns true if a stash entry was created.
func StashIfDirty(worktreePath string) bool {
	out, _ := exec.Command("git", "-C", worktreePath, "status", "--porcelain").Output()
	if len(strings.TrimSpace(string(out))) == 0 {
		return false
	}
	err := exec.Command("git", "-C", worktreePath, "stash", "--include-untracked").Run()
	return err == nil
}

// StashPop restores the most recent stash entry.
// Errors are logged at warn level but are not fatal.
func StashPop(worktreePath string) {
	out, err := exec.Command("git", "-C", worktreePath, "stash", "pop").CombinedOutput()
	if err != nil {
		slog.Default().With("component", "git").Warn("stash pop failed",
			"path", worktreePath, "error", err, "output", string(out))
	}
}
