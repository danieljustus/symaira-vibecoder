package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

// setupTestGitRepo creates a temporary directory with a minimal git repo and
// returns its absolute path.
func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "test@test")
	run("git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-m", "initial")
	return dir
}

func TestPrepareWorkspaceShared(t *testing.T) {
	ctx := context.Background()
	cycle := &config.Cycle{
		ID:   "test",
		Name: "Test",
	}

	info, err := PrepareWorkspace(ctx, cycle, "/tmp")
	if err != nil {
		t.Fatalf("PrepareWorkspace(shared): %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.Mode != "shared" {
		t.Fatalf("mode = %q, want shared", info.Mode)
	}
	if info.Path != "/tmp" {
		t.Fatalf("path = %q, want /tmp", info.Path)
	}
}

func TestPrepareWorkspaceIsolated_RequiresProjectRoot(t *testing.T) {
	ctx := context.Background()
	cycle := &config.Cycle{
		ID:        "test",
		Name:      "Test",
		Workspace: config.WorkspaceConfig{Mode: config.WorkspaceIsolated},
	}
	_, err := PrepareWorkspace(ctx, cycle, "")
	if err == nil {
		t.Fatal("expected error for empty project root, got nil")
	}
}

func TestPrepareWorkspaceIsolated_NotAGitRepo(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cycle := &config.Cycle{
		ID:        "test",
		Name:      "Test",
		Workspace: config.WorkspaceConfig{Mode: config.WorkspaceIsolated},
	}
	_, err := PrepareWorkspace(ctx, cycle, tmpDir)
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrepareWorkspaceIsolated_UnignoredWorktreeDir(t *testing.T) {
	ctx := context.Background()
	repoDir := setupTestGitRepo(t)

	cycle := &config.Cycle{
		ID:        "test",
		Name:      "Test",
		Workspace: config.WorkspaceConfig{Mode: config.WorkspaceIsolated},
	}
	_, err := PrepareWorkspace(ctx, cycle, repoDir)
	if err == nil {
		t.Fatal("expected error for unignored worktree dir, got nil")
	}
	if !strings.Contains(err.Error(), "not git-ignored") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrepareWorkspaceIsolated_DirtyCheckout(t *testing.T) {
	ctx := context.Background()
	repoDir := setupTestGitRepo(t)

	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	run("git", "add", ".gitignore")
	run("git", "commit", "-m", "add gitignore")

	// Create a dirty file.
	if err := os.WriteFile(filepath.Join(repoDir, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}

	cycle := &config.Cycle{
		ID:        "test",
		Name:      "Test",
		Workspace: config.WorkspaceConfig{Mode: config.WorkspaceIsolated},
	}
	_, err := PrepareWorkspace(ctx, cycle, repoDir)
	if err == nil {
		t.Fatal("expected error for dirty checkout, got nil")
	}
	if !strings.Contains(err.Error(), "dirty") && !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("unexpected error (want dirty-related): %v", err)
	}
}

func TestPrepareWorkspaceIsolated_Success(t *testing.T) {
	ctx := context.Background()
	repoDir := setupTestGitRepo(t)

	gitignoreContent := []byte(".worktrees/\n")
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), gitignoreContent, 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	run("git", "add", ".gitignore")
	run("git", "commit", "-m", "add gitignore")

	cycle := &config.Cycle{
		ID:        "test-cycle",
		Name:      "Test",
		Workspace: config.WorkspaceConfig{Mode: config.WorkspaceIsolated},
	}
	info, err := PrepareWorkspace(ctx, cycle, repoDir)
	if err != nil {
		t.Fatalf("PrepareWorkspace(isolated): %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.Mode != "isolated" {
		t.Fatalf("mode = %q, want isolated", info.Mode)
	}
	if info.Path == "" {
		t.Fatal("expected non-empty path")
	}
	if info.Branch == "" {
		t.Fatal("expected non-empty branch")
	}
	// Verify the worktree directory exists and is a git repo.
	if _, err := os.Stat(info.Path); os.IsNotExist(err) {
		t.Fatalf("worktree path %s does not exist", info.Path)
	}
	if !isGitRepo(info.Path) {
		t.Fatalf("worktree %s is not a git repo", info.Path)
	}
	// Verify the worktree shows up in git worktree list.
	if !isLinkedWorktree(repoDir, info.Path) {
		t.Fatalf("%s is not a linked worktree of %s", info.Path, repoDir)
	}
	// Cleanup.
	CleanupWorkspace(repoDir, info)
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Fatalf("worktree %s should have been removed", info.Path)
	}
}

func TestPrepareWorkspaceIsolated_ReuseExisting(t *testing.T) {
	ctx := context.Background()
	repoDir := setupTestGitRepo(t)

	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(".worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	run("git", "add", ".gitignore")
	run("git", "commit", "-m", "add gitignore")

	cycle := &config.Cycle{
		ID:        "reuse-test",
		Name:      "Test",
		Workspace: config.WorkspaceConfig{Mode: config.WorkspaceIsolated},
	}
	info1, err := PrepareWorkspace(ctx, cycle, repoDir)
	if err != nil {
		t.Fatalf("first PrepareWorkspace: %v", err)
	}
	if info1 == nil {
		t.Fatal("expected non-nil info from first run")
	}
	info2, err := PrepareWorkspace(ctx, cycle, repoDir)
	if err != nil {
		t.Fatalf("second PrepareWorkspace: %v", err)
	}
	if info2 == nil {
		t.Fatal("expected non-nil info from second run")
	}
	if info2.Path != info1.Path {
		t.Fatalf("second run path = %s, want %s (reuse)", info2.Path, info1.Path)
	}
	CleanupWorkspace(repoDir, info2)
}

func TestEngineWorkspaceInRunState(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "ws-state-test"
	cycle := &config.Cycle{
		ID:   cfg.Defaults.Cycle,
		Name: "WS State Test",
		Phases: []config.Phase{{
			ID: "p1", Name: "P1", Order: 1,
			Steps: []config.Step{{ID: "s1", Name: "S1", Order: 1, Enabled: true}},
		}},
	}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &countingRunner{}
	eng := New(cfg, config.NewResolver(cfg), run, NewBus())

	ws := eng.WorkspaceInfo()
	if ws != nil {
		t.Fatalf("expected nil workspace before run, got %+v", ws)
	}
	st := eng.State()
	if st.Workspace != nil {
		t.Fatalf("expected nil workspace in state before run, got %+v", st.Workspace)
	}
}

func TestEngineWorkspaceIntegration(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "ws-integration-test"
	cycle := &config.Cycle{
		ID:   cfg.Defaults.Cycle,
		Name: "WS Integration",
		Phases: []config.Phase{{
			ID: "p1", Name: "P1", Order: 1,
			Steps: []config.Step{{ID: "s1", Name: "S1", Order: 1, Enabled: true}},
		}},
	}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &countingRunner{}
	eng := New(cfg, config.NewResolver(cfg), run, NewBus())

	if _, err := eng.StartCycle(); err != nil {
		t.Fatalf("StartCycle: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for eng.State().State != "idle" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if eng.State().State != "idle" {
		t.Fatal("engine did not become idle")
	}
}
