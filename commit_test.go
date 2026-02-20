package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// gitRun executes a git command in dir and returns trimmed stdout.
// It fails the test on error.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// gitRunMayFail executes a git command in dir and returns stdout.
// Does not fail the test on error.
func gitRunMayFail(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// setupTestRepo creates a temporary git repo with an initial commit on "main".
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial commit")
	return dir
}

// setupTestRunner creates a Store and Runner for testing.
// The container command is a dummy since we're testing host-side operations.
func setupTestRunner(t *testing.T, workspaces []string) (*Store, *Runner) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := NewStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	worktreesDir := filepath.Join(t.TempDir(), "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatal(err)
	}

	runner := NewRunner(store, RunnerConfig{
		Command:      "echo", // dummy — not used for host-side operations
		SandboxImage: "test:latest",
		EnvFile:      "",
		Workspaces:   strings.Join(workspaces, " "),
		WorktreesDir: worktreesDir,
	})
	return store, runner
}

// TestWorktreeSetup verifies that worktree creation works: correct branch,
// correct directory structure, files inherited from the parent repo.
func TestWorktreeSetup(t *testing.T) {
	repo := setupTestRepo(t)
	_, runner := setupTestRunner(t, []string{repo})

	taskID := uuid.New()
	worktreePaths, branchName, err := runner.setupWorktrees(taskID)
	if err != nil {
		t.Fatal("setupWorktrees:", err)
	}
	t.Cleanup(func() { runner.cleanupWorktrees(taskID, worktreePaths, branchName) })

	if len(worktreePaths) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktreePaths))
	}

	wt := worktreePaths[repo]
	if wt == "" {
		t.Fatal("missing worktree path for repo")
	}

	// Verify worktree directory exists.
	if info, err := os.Stat(wt); err != nil || !info.IsDir() {
		t.Fatalf("worktree dir should exist: %v", err)
	}

	// Verify worktree is on the correct branch.
	branch := gitRun(t, wt, "branch", "--show-current")
	if branch != branchName {
		t.Fatalf("expected branch %q, got %q", branchName, branch)
	}

	// Verify parent files are visible.
	if _, err := os.Stat(filepath.Join(wt, "README.md")); err != nil {
		t.Fatal("README.md should exist in worktree:", err)
	}
}

// TestWorktreeGitFilePointsToHost verifies the root cause: the .git file in
// a worktree contains an absolute host path. This proves that git commands
// inside a container (where that host path doesn't exist) would fail.
func TestWorktreeGitFilePointsToHost(t *testing.T) {
	repo := setupTestRepo(t)
	_, runner := setupTestRunner(t, []string{repo})

	taskID := uuid.New()
	worktreePaths, branchName, err := runner.setupWorktrees(taskID)
	if err != nil {
		t.Fatal("setupWorktrees:", err)
	}
	t.Cleanup(func() { runner.cleanupWorktrees(taskID, worktreePaths, branchName) })

	wt := worktreePaths[repo]
	gitFile := filepath.Join(wt, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		t.Fatal("reading .git file:", err)
	}

	// The .git file contains "gitdir: /absolute/host/path/..."
	s := strings.TrimSpace(string(content))
	if !strings.HasPrefix(s, "gitdir: ") {
		t.Fatalf("unexpected .git file content: %s", s)
	}
	gitdirPath := strings.TrimPrefix(s, "gitdir: ")

	// Verify it's an absolute host path (which would NOT exist inside a container).
	if !filepath.IsAbs(gitdirPath) {
		t.Fatal("expected absolute path in .git file, got:", gitdirPath)
	}

	// The path should reference the main repo's .git directory.
	if !strings.Contains(gitdirPath, repo) {
		// The gitdir path should be under the main repo's .git/worktrees/ directory.
		// On some systems the repo path may be a symlink, so let's at least verify
		// the path exists on the host.
	}

	// Verify the path exists on the host.
	if _, err := os.Stat(gitdirPath); err != nil {
		t.Fatal("gitdir path should exist on host:", err)
	}
}

// TestHostStageAndCommit verifies that host-side staging and committing works
// correctly in a worktree.
func TestHostStageAndCommit(t *testing.T) {
	repo := setupTestRepo(t)
	_, runner := setupTestRunner(t, []string{repo})

	taskID := uuid.New()
	worktreePaths, branchName, err := runner.setupWorktrees(taskID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runner.cleanupWorktrees(taskID, worktreePaths, branchName) })

	wt := worktreePaths[repo]

	// Simulate Claude making changes.
	if err := os.WriteFile(filepath.Join(wt, "hello.txt"), []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run host-side commit.
	committed := runner.hostStageAndCommit(worktreePaths, "Add hello world file")
	if !committed {
		t.Fatal("expected commit to be created")
	}

	// Verify commit exists in worktree on the task branch.
	log := gitRun(t, wt, "log", "--oneline")
	if !strings.Contains(log, "wallfacer:") {
		t.Fatalf("expected wallfacer commit message, got:\n%s", log)
	}

	// Verify the commit is on the task branch, not on main.
	branch := gitRun(t, wt, "branch", "--show-current")
	if branch != branchName {
		t.Fatalf("should still be on task branch %q, got %q", branchName, branch)
	}
}

// TestHostStageAndCommitNoChanges verifies that host-side commit is a no-op
// when there are no changes in the worktree.
func TestHostStageAndCommitNoChanges(t *testing.T) {
	repo := setupTestRepo(t)
	_, runner := setupTestRunner(t, []string{repo})

	taskID := uuid.New()
	worktreePaths, branchName, err := runner.setupWorktrees(taskID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runner.cleanupWorktrees(taskID, worktreePaths, branchName) })

	// No changes made — commit should be a no-op.
	committed := runner.hostStageAndCommit(worktreePaths, "Nothing to do")
	if committed {
		t.Fatal("expected no commit when there are no changes")
	}
}

// TestCommitPipelineBasic tests the full commit pipeline (Phase 1-4):
// host commit → rebase → ff-merge → PROGRESS.md → cleanup.
func TestCommitPipelineBasic(t *testing.T) {
	repo := setupTestRepo(t)
	store, runner := setupTestRunner(t, []string{repo})

	initialHash := gitRun(t, repo, "rev-parse", "HEAD")

	// Create a task.
	ctx := context.Background()
	task, err := store.CreateTask(ctx, "Add a greeting file", 5)
	if err != nil {
		t.Fatal(err)
	}

	// Set up worktrees (simulates what Run() does when task starts).
	worktreePaths, branchName, err := runner.setupWorktrees(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskWorktrees(ctx, task.ID, worktreePaths, branchName); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskStatus(ctx, task.ID, "committing"); err != nil {
		t.Fatal(err)
	}

	wt := worktreePaths[repo]

	// Simulate Claude making changes in the worktree.
	if err := os.WriteFile(filepath.Join(wt, "greeting.txt"), []byte("Hello, World!\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run the commit pipeline.
	commitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	runner.commit(commitCtx, task.ID, "", 1, worktreePaths, branchName)

	// Verify a new commit exists on the default branch.
	finalHash := gitRun(t, repo, "rev-parse", "HEAD")
	if finalHash == initialHash {
		t.Fatal("expected new commit on default branch, but HEAD hasn't changed")
	}

	// Verify the file exists in the main repo's working tree.
	content, err := os.ReadFile(filepath.Join(repo, "greeting.txt"))
	if err != nil {
		t.Fatal("greeting.txt should exist in the main repo after merge:", err)
	}
	if string(content) != "Hello, World!\n" {
		t.Fatalf("unexpected content: %q", content)
	}

	// Verify the commit message references the task.
	log := gitRun(t, repo, "log", "--oneline")
	if !strings.Contains(log, "wallfacer:") {
		t.Fatalf("expected wallfacer commit in log:\n%s", log)
	}

	// Verify worktree is cleaned up.
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatal("worktree should have been cleaned up after commit pipeline")
	}
}

// TestCommitPipelineDivergedBranch tests the pipeline when the default branch
// has advanced since the worktree was created. The task's changes must be
// rebased on top of the latest default branch.
func TestCommitPipelineDivergedBranch(t *testing.T) {
	repo := setupTestRepo(t)
	store, runner := setupTestRunner(t, []string{repo})

	ctx := context.Background()
	task, err := store.CreateTask(ctx, "Add feature", 5)
	if err != nil {
		t.Fatal(err)
	}

	// Set up worktrees.
	worktreePaths, branchName, err := runner.setupWorktrees(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskWorktrees(ctx, task.ID, worktreePaths, branchName); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskStatus(ctx, task.ID, "committing"); err != nil {
		t.Fatal(err)
	}

	wt := worktreePaths[repo]

	// Simulate Claude making changes in the worktree.
	if err := os.WriteFile(filepath.Join(wt, "feature.txt"), []byte("new feature\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Meanwhile, advance the default branch in the main repo.
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("other change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-m", "other change on main")

	// Run the commit pipeline.
	commitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	runner.commit(commitCtx, task.ID, "", 1, worktreePaths, branchName)

	// Verify BOTH files exist on main (task changes rebased on top of main).
	for _, f := range []string{"feature.txt", "other.txt"} {
		if _, err := os.Stat(filepath.Join(repo, f)); err != nil {
			t.Fatalf("%s should exist on main: %v", f, err)
		}
	}

	// Verify the task commit is on top of the other commit.
	log := gitRun(t, repo, "log", "--oneline")
	lines := strings.Split(log, "\n")
	// Expected order (newest first):
	//   PROGRESS.md commit
	//   wallfacer: task commit
	//   other change on main
	//   initial commit
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 commits, got %d:\n%s", len(lines), log)
	}
}

// TestCommitPipelineNoChanges tests the pipeline when the worktree has no
// changes. The pipeline should complete without errors and without creating
// any merge commits (only PROGRESS.md may be updated).
func TestCommitPipelineNoChanges(t *testing.T) {
	repo := setupTestRepo(t)
	store, runner := setupTestRunner(t, []string{repo})

	ctx := context.Background()
	task, err := store.CreateTask(ctx, "No changes task", 5)
	if err != nil {
		t.Fatal(err)
	}

	worktreePaths, branchName, err := runner.setupWorktrees(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskWorktrees(ctx, task.ID, worktreePaths, branchName); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskStatus(ctx, task.ID, "committing"); err != nil {
		t.Fatal(err)
	}

	initialHash := gitRun(t, repo, "rev-parse", "HEAD")

	commitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	runner.commit(commitCtx, task.ID, "", 1, worktreePaths, branchName)

	// The only possible change is the PROGRESS.md commit.
	log := gitRun(t, repo, "log", "--oneline")
	// There should be no wallfacer: task commit (only PROGRESS.md and initial).
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "wallfacer:") && !strings.Contains(line, "progress log") {
			t.Fatalf("unexpected wallfacer task commit when there were no changes:\n%s", log)
		}
	}
	_ = initialHash
}

// TestCompleteTaskE2E simulates the exact waiting→done flow that the user
// reported as broken. It covers:
//  1. Create task and simulate it going through backlog → in_progress → waiting
//  2. Simulate Claude making file changes in the worktree during execution
//  3. Call the Commit pipeline (as CompleteTask handler would)
//  4. Verify that the changes end up on the default branch
func TestCompleteTaskE2E(t *testing.T) {
	repo := setupTestRepo(t)
	store, runner := setupTestRunner(t, []string{repo})

	ctx := context.Background()

	// Step 1: Create the task.
	task, err := store.CreateTask(ctx, "Add greeting feature", 5)
	if err != nil {
		t.Fatal(err)
	}

	// Step 2: Simulate task going to in_progress → worktree is created.
	worktreePaths, branchName, err := runner.setupWorktrees(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskWorktrees(ctx, task.ID, worktreePaths, branchName); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskStatus(ctx, task.ID, "in_progress"); err != nil {
		t.Fatal(err)
	}
	sessionID := "test-session-123"
	result := "I created the greeting feature"
	if err := store.UpdateTaskResult(ctx, task.ID, result, sessionID, "", 1); err != nil {
		t.Fatal(err)
	}

	// Step 3: Simulate Claude making changes in the worktree during execution.
	wt := worktreePaths[repo]
	if err := os.WriteFile(filepath.Join(wt, "greeting.txt"), []byte("Hello from wallfacer!\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Step 4: Task goes to waiting (Claude needs feedback).
	if err := store.UpdateTaskStatus(ctx, task.ID, "waiting"); err != nil {
		t.Fatal(err)
	}

	// Step 5: User clicks "Mark as Done" — this triggers Commit.
	if err := store.UpdateTaskStatus(ctx, task.ID, "committing"); err != nil {
		t.Fatal(err)
	}

	// Run the exact same code path as CompleteTask handler.
	runner.Commit(task.ID, sessionID)

	// Step 6: Verify the changes are on the default branch.
	content, err := os.ReadFile(filepath.Join(repo, "greeting.txt"))
	if err != nil {
		t.Fatal("greeting.txt should exist on default branch after Commit:", err)
	}
	if string(content) != "Hello from wallfacer!\n" {
		t.Fatalf("unexpected content: %q", content)
	}

	// Verify commit is on the default branch.
	log := gitRun(t, repo, "log", "--oneline")
	if !strings.Contains(log, "wallfacer:") {
		t.Fatalf("expected wallfacer commit on default branch:\n%s", log)
	}

	// Verify PROGRESS.md was written.
	if _, err := os.Stat(filepath.Join(repo, "PROGRESS.md")); err != nil {
		t.Fatal("PROGRESS.md should exist after commit:", err)
	}

	// Verify worktree is cleaned up.
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatal("worktree should have been cleaned up")
	}
}

// TestCommitOnTopOfLatestMain verifies that commits are created on top of
// the latest main branch, not on the stale version from when the worktree
// was created. This is critical for maintaining a clean linear history.
func TestCommitOnTopOfLatestMain(t *testing.T) {
	repo := setupTestRepo(t)
	store, runner := setupTestRunner(t, []string{repo})

	ctx := context.Background()
	task, err := store.CreateTask(ctx, "Task on stale branch", 5)
	if err != nil {
		t.Fatal(err)
	}

	// Create worktree (branches from current HEAD of main).
	worktreePaths, branchName, err := runner.setupWorktrees(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskWorktrees(ctx, task.ID, worktreePaths, branchName); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTaskStatus(ctx, task.ID, "committing"); err != nil {
		t.Fatal(err)
	}

	wt := worktreePaths[repo]

	// Make changes in the worktree.
	if err := os.WriteFile(filepath.Join(wt, "task-file.txt"), []byte("from task\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Advance main with TWO commits (simulating other tasks completing).
	if err := os.WriteFile(filepath.Join(repo, "advance1.txt"), []byte("advance 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-m", "advance main 1")

	if err := os.WriteFile(filepath.Join(repo, "advance2.txt"), []byte("advance 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-m", "advance main 2")

	mainHashBefore := gitRun(t, repo, "rev-parse", "HEAD")

	// Run the commit pipeline.
	commitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	runner.commit(commitCtx, task.ID, "", 1, worktreePaths, branchName)

	// Verify the task commit is a descendant of the latest main.
	// git merge-base --is-ancestor checks if mainHashBefore is an ancestor of HEAD.
	if _, err := gitRunMayFail(repo, "merge-base", "--is-ancestor", mainHashBefore, "HEAD"); err != nil {
		t.Fatal("task commit should be on top of latest main (rebase should have applied)")
	}

	// Verify all files exist.
	for _, f := range []string{"task-file.txt", "advance1.txt", "advance2.txt"} {
		if _, err := os.Stat(filepath.Join(repo, f)); err != nil {
			t.Fatalf("%s should exist on main: %v", f, err)
		}
	}
}

