package instructions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Key
// ---------------------------------------------------------------------------

// TestInstructionsKeyStable verifies that the same workspace list always
// produces the same key.
func TestInstructionsKeyStable(t *testing.T) {
	ws := []string{"/home/user/projectA", "/home/user/projectB"}
	k1 := Key(ws)
	k2 := Key(ws)
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
	if Key(ws1) != Key(ws2) {
		t.Fatalf("key must be order-independent: %q != %q", Key(ws1), Key(ws2))
	}
}

// TestInstructionsKeyDifferentWorkspaces verifies that distinct workspace sets
// produce distinct keys.
func TestInstructionsKeyDifferentWorkspaces(t *testing.T) {
	k1 := Key([]string{"/home/user/foo"})
	k2 := Key([]string{"/home/user/bar"})
	if k1 == k2 {
		t.Fatalf("different workspaces should produce different keys, both got %q", k1)
	}
}

// TestInstructionsKeyLength verifies the key is exactly 16 hex characters.
func TestInstructionsKeyLength(t *testing.T) {
	k := Key([]string{"/some/path"})
	if len(k) != 16 {
		t.Fatalf("expected 16-char key, got %d chars: %q", len(k), k)
	}
}

// ---------------------------------------------------------------------------
// BuildContent
// ---------------------------------------------------------------------------

// TestBuildInstructionsContentDefault verifies that when no workspace
// CLAUDE.md files exist the output is exactly the default template.
func TestBuildInstructionsContentDefault(t *testing.T) {
	dir := t.TempDir() // no CLAUDE.md inside
	content := BuildContent([]string{dir})
	if content != defaultTemplate {
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

	content := BuildContent([]string{dir})

	if !strings.HasPrefix(content, defaultTemplate) {
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
	content := BuildContent([]string{dir})
	if content != defaultTemplate {
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

	content := BuildContent([]string{dirA, dirB, dirC})

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

	content := BuildContent([]string{dir})

	if !strings.HasSuffix(content, "\n") {
		t.Fatal("content should end with a newline even when CLAUDE.md lacks one")
	}
}

// ---------------------------------------------------------------------------
// Ensure
// ---------------------------------------------------------------------------

// TestEnsureWorkspaceInstructionsCreatesFile verifies that the function
// creates a new instructions file when one does not exist yet.
func TestEnsureWorkspaceInstructionsCreatesFile(t *testing.T) {
	configDir := t.TempDir()
	ws := t.TempDir()

	path, err := Ensure(configDir, []string{ws})
	if err != nil {
		t.Fatal("Ensure:", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("instructions file should exist at %q: %v", path, err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Workspace Instructions") {
		t.Fatalf("instructions file should contain default template, got:\n%s", data)
	}
}

// TestEnsureWorkspaceInstructionsIdempotent verifies that calling Ensure a
// second time does NOT overwrite manually edited content.
func TestEnsureWorkspaceInstructionsIdempotent(t *testing.T) {
	configDir := t.TempDir()
	ws := t.TempDir()

	path, err := Ensure(configDir, []string{ws})
	if err != nil {
		t.Fatal(err)
	}

	customContent := "# My custom instructions\n"
	if err := os.WriteFile(path, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Calling again should not overwrite the custom content.
	path2, err := Ensure(configDir, []string{ws})
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

	path, err := Ensure(configDir, []string{ws})
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), repoInstructions) {
		t.Fatalf("instructions file should include workspace CLAUDE.md content; got:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// Reinit
// ---------------------------------------------------------------------------

// TestReinitWorkspaceInstructionsOverwrites verifies that Reinit replaces any
// previously written (or manually edited) content.
func TestReinitWorkspaceInstructionsOverwrites(t *testing.T) {
	configDir := t.TempDir()
	ws := t.TempDir()

	// First write stale content.
	path, err := Ensure(configDir, []string{ws})
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

	path2, err := Reinit(configDir, []string{ws})
	if err != nil {
		t.Fatal(err)
	}
	if path != path2 {
		t.Fatalf("path should be stable: %q vs %q", path, path2)
	}

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "stale content") {
		t.Fatal("Reinit should have overwritten stale content")
	}
	if !strings.Contains(string(data), repoInstructions) {
		t.Fatalf("Reinit should include fresh workspace CLAUDE.md; got:\n%s", data)
	}
}
