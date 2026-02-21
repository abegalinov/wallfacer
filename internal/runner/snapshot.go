package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"changkun.de/wallfacer/internal/logger"
)

// setupNonGitSnapshot copies ws into snapshotPath and initialises a local git
// repo there for change tracking. This lets the standard commit pipeline work
// on non-git workspaces: Phase 1 commits changes in the snapshot, Phase 2
// copies the snapshot back to ws (instead of rebasing into a remote branch).
func setupNonGitSnapshot(ws, snapshotPath string) error {
	if err := os.MkdirAll(snapshotPath, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	// Copy all files (including hidden) from ws into the snapshot.
	// The trailing "/." on the source ensures hidden files are included.
	if out, err := exec.Command("cp", "-a", ws+"/.", snapshotPath).CombinedOutput(); err != nil {
		os.RemoveAll(snapshotPath)
		return fmt.Errorf("cp workspace to snapshot: %w\n%s", err, out)
	}
	// Initialise a git repo so Phase 1 (hostStageAndCommit) can commit changes.
	if out, err := exec.Command("git", "-C", snapshotPath, "init").CombinedOutput(); err != nil {
		os.RemoveAll(snapshotPath)
		return fmt.Errorf("git init snapshot: %w\n%s", err, out)
	}
	exec.Command("git", "-C", snapshotPath, "config", "user.email", "wallfacer@local").Run()
	exec.Command("git", "-C", snapshotPath, "config", "user.name", "Wallfacer").Run()
	exec.Command("git", "-C", snapshotPath, "add", "-A").Run()
	// --allow-empty handles the edge case of an empty workspace.
	exec.Command("git", "-C", snapshotPath, "commit", "--allow-empty", "-m", "wallfacer: initial snapshot").Run()
	return nil
}

// extractSnapshotToWorkspace copies all changes from snapshotPath back to
// the original workspace at targetPath, excluding the .git directory that was
// added for change tracking. Uses rsync when available (handles deletions);
// falls back to cp which covers new/modified files only.
func extractSnapshotToWorkspace(snapshotPath, targetPath string) error {
	// rsync handles new, modified, AND deleted files correctly.
	// --checksum is needed because files may have the same size and mtime
	// but different content (e.g. macOS openrsync skips them otherwise).
	if _, err := exec.LookPath("rsync"); err == nil {
		out, err := exec.Command(
			"rsync", "-a", "--checksum", "--delete", "--exclude=.git",
			snapshotPath+"/", targetPath+"/",
		).CombinedOutput()
		if err != nil {
			return fmt.Errorf("rsync snapshot to workspace: %w\n%s", err, out)
		}
		return nil
	}
	// Fallback: cp covers new/modified files; files deleted inside the sandbox
	// are not removed from the original workspace.
	logger.Runner.Warn("rsync not found; falling back to cp (deletions will not propagate to workspace)",
		"snapshot", snapshotPath, "target", targetPath)
	if out, err := exec.Command("cp", "-a", snapshotPath+"/.", targetPath).CombinedOutput(); err != nil {
		return fmt.Errorf("cp snapshot to workspace: %w\n%s", err, out)
	}
	// Remove the .git directory that cp may have brought over from the snapshot.
	os.RemoveAll(filepath.Join(targetPath, ".git"))
	return nil
}
