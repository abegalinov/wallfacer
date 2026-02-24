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
// If defaultBranchOverride is non-empty, it is used instead of auto-detecting.
func RebaseOntoDefault(repoPath, worktreePath, defaultBranchOverride string) error {
	defBranch, err := DefaultBranchWithOverride(repoPath, defaultBranchOverride)
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
// If defaultBranchOverride is non-empty, it is used instead of auto-detecting.
func FFMerge(repoPath, branchName, defaultBranchOverride string) error {
	defBranch, err := DefaultBranchWithOverride(repoPath, defaultBranchOverride)
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
// If defaultBranchOverride is non-empty, it is used instead of auto-detecting.
func CommitsBehind(repoPath, worktreePath, defaultBranchOverride string) (int, error) {
	defBranch, err := DefaultBranchWithOverride(repoPath, defaultBranchOverride)
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

// MergeBase returns the best common ancestor (merge-base) of two refs,
// evaluated in the given repository/worktree path.
func MergeBase(repoPath, ref1, ref2 string) (string, error) {
	out, err := exec.Command("git", "-C", repoPath, "merge-base", ref1, ref2).Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base %s %s in %s: %w", ref1, ref2, repoPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsConflictOutput reports whether git output text indicates a merge conflict.
func IsConflictOutput(s string) bool {
	return strings.Contains(s, "CONFLICT") ||
		strings.Contains(s, "Merge conflict") ||
		strings.Contains(s, "conflict")
}
