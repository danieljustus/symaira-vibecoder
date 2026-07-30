package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

// WorkspaceInfo describes the active workspace for a run. For shared mode only
// the Path is set; for isolated mode Branch and WorktreeDir are also populated.
type WorkspaceInfo struct {
	Path        string `json:"path"`             // absolute working directory
	Mode        string `json:"mode"`             // "shared" | "isolated"
	Branch      string `json:"branch,omitempty"` // worktree branch (isolated only)
	WorktreeDir string `json:"worktree_dir,omitempty"`
	CleanupFn   func() `json:"-"` // cleanup callback, set when isolated
}

// ErrDirtySourceCheckout is returned when the source checkout has uncommitted
// changes and isolated mode cannot safely create a worktree.
var ErrDirtySourceCheckout = errors.New("workspace: source checkout is dirty — commit or stash before isolated run")

// ErrUnignoredWorktreeDir is returned when .worktrees/ (or the configured
// worktree dir) is not in .gitignore.
var ErrUnignoredWorktreeDir = errors.New("workspace: worktree directory is not git-ignored")

// ErrWorktreeAlreadyExists is returned when a linked worktree for the cycle
// already exists on a different branch or in an unexpected state.
var ErrWorktreeAlreadyExists = errors.New("workspace: linked worktree already exists for this cycle")

// errNotAGitRepo is used internally when the source checkout is not a git repo.
var errNotAGitRepo = errors.New("workspace: source checkout is not a git repository")

// PrepareWorkspace validates and sets up the workspace for a cycle run.
// For shared mode it simply verifies the source directory exists.
// For isolated mode it creates a linked worktree.
// It calls config.WorkspaceConfig.Validate internally.
func PrepareWorkspace(ctx context.Context, cycle *config.Cycle, projectRoot string) (*WorkspaceInfo, error) {
	if err := cycle.Workspace.Validate(); err != nil {
		return nil, err
	}

	mode := cycle.Workspace.EffectiveMode()
	switch mode {
	case config.WorkspaceShared:
		info := &WorkspaceInfo{
			Path: projectRoot,
			Mode: "shared",
		}
		// Accept empty projectRoot (use current process dir) but prefer the
		// project root when given.
		return info, nil

	case config.WorkspaceIsolated:
		return prepareIsolated(ctx, cycle, projectRoot)

	default:
		return nil, fmt.Errorf("workspace: unhandled mode %q", mode)
	}
}

// prepareIsolated creates or reuses a linked worktree for the cycle.
func prepareIsolated(ctx context.Context, cycle *config.Cycle, projectRoot string) (*WorkspaceInfo, error) {
	if projectRoot == "" {
		return nil, errors.New("workspace: project root required for isolated mode")
	}

	// 1. Verify the project root is a git repository.
	if !isGitRepo(projectRoot) {
		return nil, fmt.Errorf("%w: %s", errNotAGitRepo, projectRoot)
	}

	// 2. Determine the worktree directory.
	wtDir := cycle.Workspace.WorktreeDir
	if wtDir == "" {
		wtDir = filepath.Join(projectRoot, ".worktrees")
	} else if !filepath.IsAbs(wtDir) {
		wtDir = filepath.Join(projectRoot, wtDir)
	}
	wtDir = filepath.Clean(wtDir)

	// 3. Check that the worktree directory is git-ignored.
	ignored, err := isGitIgnored(projectRoot, wtDir)
	if err != nil {
		return nil, fmt.Errorf("workspace: check gitignore: %w", err)
	}
	if !ignored {
		return nil, ErrUnignoredWorktreeDir
	}

	// 4. Check that the source checkout is clean (no uncommitted changes).
	clean, err := isGitClean(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("workspace: check git status: %w", err)
	}
	if !clean {
		return nil, ErrDirtySourceCheckout
	}

	// 5. Determine the worktree branch and path.
	// Sluggish branch name derived from the cycle id.
	branchName := "workspace/cycle-" + cycle.ID
	worktreePath := filepath.Join(wtDir, "cycle-"+cycle.ID)

	// 6. Check whether the worktree already exists.
	if _, err := os.Stat(worktreePath); err == nil {
		// A directory exists — verify it is the correct worktree.
		if !isGitRepo(worktreePath) {
			return nil, fmt.Errorf("workspace: %s exists but is not a git repository", worktreePath)
		}
		// Verify it's actually a linked worktree pointing to our project.
		if !isLinkedWorktree(projectRoot, worktreePath) {
			return nil, ErrWorktreeAlreadyExists
		}
		// Reuse it — ensure it is on the correct branch.
		if err := ensureBranch(ctx, worktreePath, branchName); err != nil {
			return nil, fmt.Errorf("workspace: ensure worktree branch: %w", err)
		}
	} else if os.IsNotExist(err) {
		// Create the worktree directory first.
		if err := os.MkdirAll(wtDir, 0o755); err != nil {
			return nil, fmt.Errorf("workspace: mkdir %s: %w", wtDir, err)
		}
		// Create a linked worktree.
		if err := createWorktree(ctx, projectRoot, worktreePath, branchName); err != nil {
			return nil, fmt.Errorf("workspace: create worktree: %w", err)
		}
	} else {
		return nil, fmt.Errorf("workspace: stat %s: %w", worktreePath, err)
	}

	info := &WorkspaceInfo{
		Path:        worktreePath,
		Mode:        "isolated",
		Branch:      branchName,
		WorktreeDir: wtDir,
		CleanupFn: func() {
			_ = removeWorktree(context.Background(), projectRoot, worktreePath, branchName)
		},
	}
	return info, nil
}

// CleanupWorkspace removes a previously created isolated worktree. It is a
// no-op for shared mode and logs rather than fatals on errors.
func CleanupWorkspace(projectRoot string, info *WorkspaceInfo) {
	if info == nil || info.Mode != "isolated" || info.CleanupFn == nil {
		return
	}
	info.CleanupFn()
}

// MergeWorkspace merges the isolated branch into main, then cleans up the
// worktree. It returns an error if the merge has conflicts.
func MergeWorkspace(ctx context.Context, projectRoot string, info *WorkspaceInfo) error {
	if info == nil || info.Mode != "isolated" {
		return errors.New("workspace: merge only valid for isolated mode")
	}
	// Fetch latest main.
	if err := runQuiet(ctx, projectRoot, "git", "fetch", "origin", "main"); err != nil {
		return fmt.Errorf("workspace: fetch main: %w", err)
	}
	// Checkout the worktree branch in the source checkout.
	branch := info.Branch
	worktreePath := info.Path

	// Verify the worktree branch exists.
	if err := runQuiet(ctx, projectRoot, "git", "fetch", "origin", branch); err != nil {
		// Branch may not be pushed yet — that's fine.
		_ = err
	}

	// Attempt to merge the worktree branch into main.
	// We use the worktree's branch ref.
	if err := runQuiet(ctx, worktreePath, "git", "checkout", branch); err != nil {
		return fmt.Errorf("workspace: checkout worktree branch: %w", err)
	}
	if err := runQuiet(ctx, worktreePath, "git", "merge", "origin/main",
		"--no-ff", "-m", "Merge isolated workspace branch "+branch); err != nil {
		return fmt.Errorf("workspace: merge failed (conflicts?): %w", err)
	}

	// Clean up the worktree after successful merge.
	CleanupWorkspace(projectRoot, info)
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func isGitRepo(dir string) bool {
	err := runQuiet(context.Background(), dir, "git", "rev-parse", "--git-dir")
	return err == nil
}

func isGitClean(dir string) (bool, error) {
	var out bytes.Buffer
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return false, err
	}
	return strings.TrimSpace(out.String()) == "", nil
}

func isGitIgnored(projectRoot, path string) (bool, error) {
	// git check-ignore works with paths relative to the repo root.
	rel, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return false, err
	}
	if strings.HasPrefix(rel, "..") {
		return false, nil // outside the repo — not ignored
	}
	// Always check with trailing separator: the worktree path is a directory
	// and gitignore patterns like ".worktrees/" only match directories when
	// checked with the trailing slash.
	if !strings.HasSuffix(rel, string(filepath.Separator)) {
		rel += string(filepath.Separator)
	}
	var out bytes.Buffer
	cmd := exec.Command("git", "check-ignore", rel)
	cmd.Dir = projectRoot
	cmd.Stdout = &out
	cmd.Stderr = nil
	err = cmd.Run()
	if err != nil {
		// check-ignore exits 1 when the path is NOT ignored — that's our signal.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("git check-ignore: %w", err)
	}
	return true, nil
}

func isLinkedWorktree(mainRepo, worktreePath string) bool {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = mainRepo
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// Resolve symlinks — git worktree list outputs canonical paths.
	canonicalPath, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		canonicalPath = worktreePath
	}
	prefix := "worktree "
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, prefix) {
			wt := strings.TrimSpace(line[len(prefix):])
			// Resolve the reported worktree path too.
			canonicalWT, err := filepath.EvalSymlinks(wt)
			if err == nil {
				wt = canonicalWT
			}
			if wt == canonicalPath {
				return true
			}
		}
	}
	return false
}

func createWorktree(ctx context.Context, mainRepo, worktreePath, branch string) error {
	args := []string{"worktree", "add", "-b", branch, worktreePath, "HEAD"}
	return runQuiet(ctx, mainRepo, "git", args...)
}

func ensureBranch(ctx context.Context, worktreePath, branch string) error {
	// Check current branch.
	var out bytes.Buffer
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = worktreePath
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return err
	}
	currentBranch := strings.TrimSpace(out.String())
	if currentBranch == branch {
		return nil // already on the right branch
	}
	// Fetch and checkout the desired branch.
	if err := runQuiet(ctx, worktreePath, "git", "fetch", "origin", branch); err != nil {
		// Branch might not exist on remote — create locally.
		_ = err
	}
	return runQuiet(ctx, worktreePath, "git", "checkout", branch)
}

func removeWorktree(ctx context.Context, mainRepo, worktreePath, branch string) error {
	// Remove the worktree.
	err := runQuiet(ctx, mainRepo, "git", "worktree", "remove", "--force", worktreePath)
	if err != nil {
		return err
	}
	// Prune stale worktree references.
	_ = runQuiet(ctx, mainRepo, "git", "worktree", "prune")
	return nil
}

func runQuiet(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg != "" {
			return fmt.Errorf("%s %v: %s", name, args, msg)
		}
		return err
	}
	return nil
}
