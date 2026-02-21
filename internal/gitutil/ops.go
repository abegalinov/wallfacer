package gitutil

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// RebaseOntoDefault rebases the task branch (currently checked out in worktreePath)
// onto the default branch of repoPath. On conflict it aborts the rebase and returns
// ErrConflict so the caller can invoke conflict resolution and retry.
func RebaseOntoDefault(repoPath, worktreePath string) error {
	defBranch, err := DefaultBranch(repoPath)
	if err != nil {
		return err
	}
	out, err := exec.Command("git", "-C", worktreePath, "rebase", defBranch).CombinedOutput()
	if err != nil {
		// Abort so the repo is not stuck mid-rebase.
		exec.Command("git", "-C", worktreePath, "rebase", "--abort").Run()
		if IsConflictOutput(string(out)) {
			return fmt.Errorf("%w in %s", ErrConflict, worktreePath)
		}
		return fmt.Errorf("git rebase in %s: %w\n%s", worktreePath, err, out)
	}
	return nil
}

// FFMerge fast-forward merges branchName into the default branch of repoPath.
func FFMerge(repoPath, branchName string) error {
	defBranch, err := DefaultBranch(repoPath)
	if err != nil {
		return err
	}
	if out, err := exec.Command("git", "-C", repoPath, "checkout", defBranch).CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout %s in %s: %w\n%s", defBranch, repoPath, err, out)
	}
	out, err := exec.Command("git", "-C", repoPath, "merge", "--ff-only", branchName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git merge --ff-only %s in %s: %w\n%s", branchName, repoPath, err, out)
	}
	return nil
}

// CommitsBehind returns the number of commits the default branch has ahead of
// the worktree's HEAD (i.e. how many commits the task branch is behind).
func CommitsBehind(repoPath, worktreePath string) (int, error) {
	defBranch, err := DefaultBranch(repoPath)
	if err != nil {
		return 0, err
	}
	out, err := exec.Command(
		"git", "-C", worktreePath,
		"rev-list", "--count", "HEAD.."+defBranch,
	).Output()
	if err != nil {
		return 0, fmt.Errorf("git rev-list in %s: %w", worktreePath, err)
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n, nil
}

// HasCommitsAheadOf reports whether worktreePath has commits not yet in baseBranch.
func HasCommitsAheadOf(worktreePath, baseBranch string) (bool, error) {
	out, err := exec.Command(
		"git", "-C", worktreePath,
		"rev-list", "--count", baseBranch+"..HEAD",
	).Output()
	if err != nil {
		return false, fmt.Errorf("git rev-list in %s: %w", worktreePath, err)
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n > 0, nil
}

// IsConflictOutput reports whether git output text indicates a merge conflict.
func IsConflictOutput(s string) bool {
	return strings.Contains(s, "CONFLICT") ||
		strings.Contains(s, "Merge conflict") ||
		strings.Contains(s, "conflict")
}
