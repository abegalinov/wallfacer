package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// instructionsKey
// ---------------------------------------------------------------------------

// TestInstructionsKeyStable verifies that the same workspace list always
// produces the same key.
func TestInstructionsKeyStable(t *testing.T) {
	ws := []string{"/home/user/projectA", "/home/user/projectB"}
	k1 := instructionsKey(ws)
	k2 := instructionsKey(ws)
	if k1 != k2 {
		t.Fatalf("key should be stable: got %q then %q", k1, k2)
	}
}

// TestInstructionsKeyOrderIndependent verifies that workspace order does not
// affect the key, so wallfacer run ~/a ~/b and wallfacer run ~/b ~/a share
// the same instructions file.
func TestInstructionsKeyOrderIndependent(t *testing.T) {
	ws1 := []string{"/home/user/alpha", "/home/user/beta"}
	ws2 := []string{"/home/user/beta", "/home/user/alpha"}
	if instructionsKey(ws1) != instructionsKey(ws2) {
		t.Fatalf("key must be order-independent: %q != %q",
			instructionsKey(ws1), instructionsKey(ws2))
	}
}

// TestInstructionsKeyDifferentWorkspaces verifies that distinct workspace sets
// produce distinct keys.
func TestInstructionsKeyDifferentWorkspaces(t *testing.T) {
	k1 := instructionsKey([]string{"/home/user/foo"})
	k2 := instructionsKey([]string{"/home/user/bar"})
	if k1 == k2 {
		t.Fatalf("different workspaces should produce different keys, both got %q", k1)
	}
}

// TestInstructionsKeyLength verifies the key is exactly 16 hex characters.
func TestInstructionsKeyLength(t *testing.T) {
	k := instructionsKey([]string{"/some/path"})
	if len(k) != 16 {
		t.Fatalf("expected 16-char key, got %d chars: %q", len(k), k)
	}
}

// ---------------------------------------------------------------------------
// buildInstructionsContent
// ---------------------------------------------------------------------------

// TestBuildInstructionsContentDefault verifies that when no workspace
// CLAUDE.md files exist the output is exactly the default template.
func TestBuildInstructionsContentDefault(t *testing.T) {
	dir := t.TempDir() // no CLAUDE.md inside
	content := buildInstructionsContent([]string{dir})
	if content != defaultInstructionsTemplate {
		t.Fatalf("expected default template only, got:\n%s", content)
	}
}

// TestBuildInstructionsContentWithWorkspaceCLAUDE verifies that a workspace
// CLAUDE.md is appended after the default template with the correct header.
func TestBuildInstructionsContentWithWorkspaceCLAUDE(t *testing.T) {
	dir := t.TempDir()
	repoInstructions := "# My project rules\n\nDo the thing.\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(repoInstructions), 0644); err != nil {
		t.Fatal(err)
	}

	content := buildInstructionsContent([]string{dir})

	if !strings.HasPrefix(content, defaultInstructionsTemplate) {
		t.Fatal("content should start with the default template")
	}

	name := filepath.Base(dir)
	expectedHeader := "\n---\n\n## Instructions from `" + name + "`\n\n"
	if !strings.Contains(content, expectedHeader) {
		t.Fatalf("expected header %q in content:\n%s", expectedHeader, content)
	}

	if !strings.Contains(content, repoInstructions) {
		t.Fatalf("expected repo instructions in content:\n%s", content)
	}
}

// TestBuildInstructionsContentMissingCLAUDE verifies that a workspace without
// a CLAUDE.md is silently skipped (no error, just default template).
func TestBuildInstructionsContentMissingCLAUDE(t *testing.T) {
	dir := t.TempDir() // no CLAUDE.md
	content := buildInstructionsContent([]string{dir})
	if content != defaultInstructionsTemplate {
		t.Fatalf("workspace without CLAUDE.md should produce only default template")
	}
}

// TestBuildInstructionsContentMultipleWorkspaces verifies that CLAUDE.md from
// several workspaces are all appended in order, each with its own header.
func TestBuildInstructionsContentMultipleWorkspaces(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	dirC := t.TempDir()

	if err := os.WriteFile(filepath.Join(dirA, "CLAUDE.md"), []byte("instructions for A\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// dirB intentionally has no CLAUDE.md

	if err := os.WriteFile(filepath.Join(dirC, "CLAUDE.md"), []byte("instructions for C\n"), 0644); err != nil {
		t.Fatal(err)
	}

	content := buildInstructionsContent([]string{dirA, dirB, dirC})

	if !strings.Contains(content, "instructions for A") {
		t.Error("missing instructions from workspace A")
	}
	if !strings.Contains(content, "instructions for C") {
		t.Error("missing instructions from workspace C")
	}

	// dirB has no CLAUDE.md — its name should not appear as a header.
	nameB := filepath.Base(dirB)
	if strings.Contains(content, "## Instructions from `"+nameB+"`") {
		t.Errorf("workspace B (no CLAUDE.md) should not produce a header")
	}

	// A's section must come before C's section.
	posA := strings.Index(content, "instructions for A")
	posC := strings.Index(content, "instructions for C")
	if posA > posC {
		t.Error("workspace A section should appear before workspace C section")
	}
}

// TestBuildInstructionsContentTrailingNewline verifies that a workspace
// CLAUDE.md without a trailing newline still results in valid content (a
// newline is appended so headers are not run together).
func TestBuildInstructionsContentTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	// Deliberately omit trailing newline.
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("no newline at end"), 0644); err != nil {
		t.Fatal(err)
	}

	content := buildInstructionsContent([]string{dir})

	if !strings.HasSuffix(content, "\n") {
		t.Fatal("content should end with a newline even when CLAUDE.md lacks one")
	}
}

// ---------------------------------------------------------------------------
// ensureWorkspaceInstructions
// ---------------------------------------------------------------------------

// TestEnsureWorkspaceInstructionsCreatesFile verifies that the function
// creates a new instructions file when one does not exist yet.
func TestEnsureWorkspaceInstructionsCreatesFile(t *testing.T) {
	configDir := t.TempDir()
	ws := t.TempDir()

	path, err := ensureWorkspaceInstructions(configDir, []string{ws})
	if err != nil {
		t.Fatal("ensureWorkspaceInstructions:", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("instructions file should exist at %q: %v", path, err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Workspace Instructions") {
		t.Fatalf("instructions file should contain default template, got:\n%s", data)
	}
}

// TestEnsureWorkspaceInstructionsIdempotent verifies that calling ensure a
// second time does NOT overwrite manually edited content.
func TestEnsureWorkspaceInstructionsIdempotent(t *testing.T) {
	configDir := t.TempDir()
	ws := t.TempDir()

	path, err := ensureWorkspaceInstructions(configDir, []string{ws})
	if err != nil {
		t.Fatal(err)
	}

	customContent := "# My custom instructions\n"
	if err := os.WriteFile(path, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Calling again should not overwrite the custom content.
	path2, err := ensureWorkspaceInstructions(configDir, []string{ws})
	if err != nil {
		t.Fatal(err)
	}
	if path != path2 {
		t.Fatalf("path changed between calls: %q vs %q", path, path2)
	}

	data, _ := os.ReadFile(path)
	if string(data) != customContent {
		t.Fatalf("existing content should be preserved; got:\n%s", data)
	}
}

// TestEnsureWorkspaceInstructionsIncludesWorkspaceCLAUDE verifies that a
// newly created instructions file incorporates the workspace's own CLAUDE.md.
func TestEnsureWorkspaceInstructionsIncludesWorkspaceCLAUDE(t *testing.T) {
	configDir := t.TempDir()
	ws := t.TempDir()

	repoInstructions := "# Project-specific rules\n"
	if err := os.WriteFile(filepath.Join(ws, "CLAUDE.md"), []byte(repoInstructions), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := ensureWorkspaceInstructions(configDir, []string{ws})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), repoInstructions) {
		t.Fatalf("instructions file should include workspace CLAUDE.md content; got:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// reinitWorkspaceInstructions
// ---------------------------------------------------------------------------

// TestReinitWorkspaceInstructionsOverwrites verifies that reinit replaces any
// previously written (or manually edited) content.
func TestReinitWorkspaceInstructionsOverwrites(t *testing.T) {
	configDir := t.TempDir()
	ws := t.TempDir()

	// First write stale content.
	path, err := ensureWorkspaceInstructions(configDir, []string{ws})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Now add a CLAUDE.md to the workspace and reinit.
	repoInstructions := "# Fresh instructions\n"
	if err := os.WriteFile(filepath.Join(ws, "CLAUDE.md"), []byte(repoInstructions), 0644); err != nil {
		t.Fatal(err)
	}

	path2, err := reinitWorkspaceInstructions(configDir, []string{ws})
	if err != nil {
		t.Fatal(err)
	}
	if path != path2 {
		t.Fatalf("path should be stable: %q vs %q", path, path2)
	}

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "stale content") {
		t.Fatal("reinit should have overwritten stale content")
	}
	if !strings.Contains(string(data), repoInstructions) {
		t.Fatalf("reinit should include fresh workspace CLAUDE.md; got:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// Runner.buildContainerArgs — CLAUDE.md mount
// ---------------------------------------------------------------------------

// newTestRunnerWithInstructions creates a Runner whose instructionsPath points
// to the given path (may or may not exist on disk).
func newTestRunnerWithInstructions(t *testing.T, instructionsPath string) *Runner {
	t.Helper()
	dataDir := t.TempDir()
	store, err := NewStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return NewRunner(store, RunnerConfig{
		Command:          "podman",
		SandboxImage:     "wallfacer:latest",
		InstructionsPath: instructionsPath,
	})
}

// TestContainerArgsMountsCLAUDEMD verifies that when instructionsPath is set
// and the file exists, buildContainerArgs includes a read-only volume mount
// that places it at /workspace/CLAUDE.md inside the container.
func TestContainerArgsMountsCLAUDEMD(t *testing.T) {
	instructionsFile := filepath.Join(t.TempDir(), "instructions.md")
	if err := os.WriteFile(instructionsFile, []byte(defaultInstructionsTemplate), 0644); err != nil {
		t.Fatal(err)
	}

	runner := newTestRunnerWithInstructions(t, instructionsFile)
	args := runner.buildContainerArgs("test-container", "do something", "", nil)

	expectedMount := instructionsFile + ":/workspace/CLAUDE.md:z,ro"
	if !containsConsecutive(args, "-v", expectedMount) {
		t.Fatalf("args should contain -v %q; got: %v", expectedMount, args)
	}
}

// TestContainerArgsNoInstructionsPath verifies that when InstructionsPath is
// empty no CLAUDE.md mount is added to the container args.
func TestContainerArgsNoInstructionsPath(t *testing.T) {
	runner := newTestRunnerWithInstructions(t, "")
	args := runner.buildContainerArgs("test-container", "do something", "", nil)

	for _, a := range args {
		if strings.Contains(a, "CLAUDE.md") {
			t.Fatalf("expected no CLAUDE.md mount when InstructionsPath is empty; got arg: %q", a)
		}
	}
}

// TestContainerArgsMissingInstructionsFile verifies that when instructionsPath
// is set but the file does not exist, no CLAUDE.md mount is added (the runner
// silently skips a missing file rather than failing the container launch).
func TestContainerArgsMissingInstructionsFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "nonexistent.md")
	runner := newTestRunnerWithInstructions(t, missingPath)
	args := runner.buildContainerArgs("test-container", "do something", "", nil)

	for _, a := range args {
		if strings.Contains(a, "CLAUDE.md") {
			t.Fatalf("expected no CLAUDE.md mount for missing file; got arg: %q", a)
		}
	}
}

// TestContainerArgsCLAUDEMDMountIsReadOnly verifies the mount is marked :ro
// so the container cannot accidentally modify the shared instructions file.
func TestContainerArgsCLAUDEMDMountIsReadOnly(t *testing.T) {
	instructionsFile := filepath.Join(t.TempDir(), "instructions.md")
	if err := os.WriteFile(instructionsFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	runner := newTestRunnerWithInstructions(t, instructionsFile)
	args := runner.buildContainerArgs("test-container", "do something", "", nil)

	for i, a := range args {
		if a == "-v" && i+1 < len(args) && strings.Contains(args[i+1], "CLAUDE.md") {
			mount := args[i+1]
			// Accept both ":ro" and ",ro" (SELinux label adds ":z,ro").
			if !strings.HasSuffix(mount, ":ro") && !strings.HasSuffix(mount, ",ro") {
				t.Fatalf("CLAUDE.md mount should be read-only, got: %q", mount)
			}
			return
		}
	}
	t.Fatal("CLAUDE.md -v mount not found in args")
}

// TestContainerArgsCLAUDEMDMountPosition verifies that the CLAUDE.md mount
// appears before the image name in the args list, matching the expected
// container launch order.
func TestContainerArgsCLAUDEMDMountPosition(t *testing.T) {
	instructionsFile := filepath.Join(t.TempDir(), "instructions.md")
	if err := os.WriteFile(instructionsFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	ws := t.TempDir()
	dataDir := t.TempDir()
	store, err := NewStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	runner := NewRunner(store, RunnerConfig{
		Command:          "podman",
		SandboxImage:     "wallfacer:latest",
		InstructionsPath: instructionsFile,
		Workspaces:       ws,
	})
	args := runner.buildContainerArgs("test-container", "do something", "", nil)

	claudeMDIdx := -1
	imageIdx := -1
	for i, a := range args {
		if strings.Contains(a, "CLAUDE.md") {
			claudeMDIdx = i
		}
		if a == "wallfacer:latest" {
			imageIdx = i
		}
	}

	if claudeMDIdx == -1 {
		t.Fatal("CLAUDE.md mount not found in args")
	}
	if imageIdx == -1 {
		t.Fatal("sandbox image not found in args")
	}
	if claudeMDIdx >= imageIdx {
		t.Fatalf("CLAUDE.md mount (index %d) should appear before sandbox image (index %d)",
			claudeMDIdx, imageIdx)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// containsConsecutive returns true if slice contains needle1 immediately
// followed by needle2.
func containsConsecutive(slice []string, needle1, needle2 string) bool {
	for i := 0; i+1 < len(slice); i++ {
		if slice[i] == needle1 && slice[i+1] == needle2 {
			return true
		}
	}
	return false
}
