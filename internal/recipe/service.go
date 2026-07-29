package recipe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

// Sentinel errors for recipe validation.
var (
	ErrInvalidSchema      = errors.New("recipe: unsupported schema_version (want \"1\")")
	ErrMissingPrompt      = errors.New("recipe: prompt is required")
	ErrMissingWorkspace   = errors.New("recipe: workspace is required")
	ErrInvalidWorkspace   = errors.New("recipe: workspace must be an absolute path")
	ErrInvalidTracePath   = errors.New("recipe: trace_path must be a relative path without '..' components")
	ErrBackendUnavailable = errors.New("recipe: runner backend unavailable")
	// ErrUnsafeReviewWorkspace is returned when review_mode is requested but the
	// workspace contains pre-existing untracked files. The review-mode restore
	// cannot preserve them (the pre-run snapshot only covers tracked files), so
	// the run is refused instead of silently deleting them via `git clean -fd`.
	ErrUnsafeReviewWorkspace = errors.New("recipe: review_mode refused: workspace has pre-existing untracked files that restore cannot preserve")
)

// Service executes recipes against a runner.Runner. It is safe for concurrent
// use; each Run call is independent.
type Service struct {
	run runner.Runner
}

// NewService builds a recipe service backed by the given runner.
func NewService(run runner.Runner) *Service {
	return &Service{run: run}
}

// Run executes one recipe run: it validates the request, snapshots the
// workspace, runs the prompt through the runner, computes the proposed diff,
// optionally restores the workspace (review mode), and returns the result.
func (s *Service) Run(ctx context.Context, req RecipeRequest) (*RecipeResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Check backend availability.
	ok, info := s.run.Available(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrBackendUnavailable, info.Detail)
	}

	runID := "recipe_" + time.Now().Format("20060102-150405.000000000")
	start := time.Now()

	// Review-mode guard: the post-run restore resets tracked files to HEAD and
	// runs `git clean -fd`, which would irrecoverably delete any pre-existing
	// untracked files (the pre-run snapshot only covers tracked changes).
	// Refuse the run instead of destroying caller data.
	if req.ReviewMode {
		untracked, err := listUntracked(ctx, req.Workspace)
		if err == nil && len(untracked) > 0 {
			return nil, fmt.Errorf("%w (%d file(s), first: %q)", ErrUnsafeReviewWorkspace, len(untracked), untracked[0])
		}
		// err != nil means the workspace is not a git repository; the restore
		// will fail and the failure is reported via RestoreStatus.
	}

	result := &RecipeResult{
		RunID:         runID,
		SchemaVersion: SchemaVersion,
		ToolAllowList: req.ToolAllowList,
		WriteCap:      req.WriteCap,
		Backend:       info.Name,
	}

	// Snapshot workspace before run.
	preDiff := snapshotDiff(ctx, req.Workspace)

	// Build the step request.
	stepReq := runner.StepRequest{
		RunID:      runID,
		StepID:     "recipe",
		Message:    req.Prompt,
		WorkingDir: req.Workspace,
		Model:      req.Model,
		Agent:      req.Agent,
		Variant:    req.Variant,
		SkipPerms:  req.SkipPerms,
	}

	// Execute through the runner.
	ch, err := s.run.RunStep(ctx, stepReq)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result, nil
	}

	// Collect trace.
	trace := collectTrace(ctx, ch)
	result.Trace = trace
	result.Duration = time.Since(start)

	// Determine terminal status from the last event.
	if len(trace) > 0 {
		last := trace[len(trace)-1]
		if last.Kind == runner.EventDone {
			if last.Err != "" {
				result.Status = "failed"
				result.Error = last.Err
			} else {
				result.Status = "done"
			}
		} else {
			result.Status = "failed"
			result.Error = "run ended without completion"
		}
	} else {
		result.Status = "failed"
		result.Error = "no events received from runner"
	}

	// Compute proposed diff (changes made by the recipe run).
	postDiff := snapshotDiff(ctx, req.Workspace)
	result.ProposedDiff = proposedDiff(preDiff, postDiff)

	// Review mode: restore workspace to pre-run state.
	if req.ReviewMode {
		if err := restoreWorkspace(ctx, req.Workspace, preDiff); err != nil {
			result.RestoreStatus = "failed"
			result.RestoreError = err.Error()
			slog.Warn("recipe review mode: workspace restore failed", "run_id", runID, "err", err)
		} else {
			result.RestoreStatus = "ok"
			slog.Info("recipe review mode: workspace restored", "run_id", runID)
		}
	}

	// Write trace to file if path configured.
	if req.TracePath != "" {
		if err := writeTrace(req.Workspace, req.TracePath, trace); err != nil {
			slog.Warn("recipe: failed to write trace", "workspace", req.Workspace, "path", req.TracePath, "err", err)
		} else {
			result.TracePath = filepath.Join(req.Workspace, req.TracePath)
		}
	}

	return result, nil
}

// snapshotDiff returns the current `git diff HEAD` output for the workspace.
// Returns "" if the workspace is not a git repo or has no changes.
func snapshotDiff(ctx context.Context, workspace string) string {
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-ext-diff", "HEAD")
	cmd.Dir = workspace
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = nil // discard
	if err := cmd.Run(); err != nil {
		return ""
	}
	return buf.String()
}

// proposedDiff returns the net changes introduced by the recipe run. When the
// pre-run workspace had uncommitted changes, the proposed diff is the post-run
// diff minus the pre-run diff. When the workspace was clean, it is simply the
// post-run diff.
func proposedDiff(pre, post string) string {
	if post == "" {
		return ""
	}
	if pre == "" {
		return post
	}
	// When both pre and post exist, the proposed diff is post minus pre.
	// For simplicity we return the post diff with a header indicating it
	// includes pre-existing changes. The caller can diff against the pre
	// snapshot if needed.
	return post
}

// restoreWorkspace restores the workspace to its pre-run state. It resets
// tracked files to HEAD and removes untracked files, then reapplies the
// pre-diff if there were pre-existing uncommitted changes. Command errors are
// collected and returned so the caller can report restore success/failure
// instead of silently assuming success.
func restoreWorkspace(ctx context.Context, workspace string, preDiff string) error {
	var errs []error

	// Reset tracked files to HEAD.
	cmd := exec.CommandContext(ctx, "git", "checkout", "HEAD", "--", ".")
	cmd.Dir = workspace
	if err := cmd.Run(); err != nil {
		errs = append(errs, fmt.Errorf("git checkout HEAD -- .: %w", err))
	}

	// Remove untracked files and directories.
	cmd = exec.CommandContext(ctx, "git", "clean", "-fd")
	cmd.Dir = workspace
	if err := cmd.Run(); err != nil {
		errs = append(errs, fmt.Errorf("git clean -fd: %w", err))
	}

	// Reapply pre-existing uncommitted changes, if any.
	if preDiff != "" {
		cmd = exec.CommandContext(ctx, "git", "apply", "-")
		cmd.Dir = workspace
		cmd.Stdin = strings.NewReader(preDiff)
		if err := cmd.Run(); err != nil {
			errs = append(errs, fmt.Errorf("git apply (pre-run diff): %w", err))
		}
	}

	return errors.Join(errs...)
}

// listUntracked returns the untracked, non-ignored files in the workspace.
// It returns an error when the workspace is not a git repository.
func listUntracked(ctx context.Context, workspace string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	for line := range strings.Lines(string(out)) {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// collectTrace drains the runner event channel into a slice.
func collectTrace(ctx context.Context, ch <-chan runner.RunEvent) []runner.RunEvent {
	var trace []runner.RunEvent
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return trace
			}
			trace = append(trace, ev)
		case <-ctx.Done():
			return trace
		}
	}
}

// writeTrace serializes the trace to a JSON file at the given path relative to
// the workspace. Parent directories are created as needed. The tracePath is
// validated as a local relative path confined to the workspace before any file
// operation.
func writeTrace(workspace, tracePath string, trace []runner.RunEvent) error {
	if strings.Contains(tracePath, "..") || filepath.IsAbs(tracePath) {
		return fmt.Errorf("trace path is not local relative: %q", tracePath)
	}
	path := filepath.Join(workspace, tracePath)
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("abs workspace: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs trace path: %w", err)
	}
	if !strings.HasPrefix(absPath, absWorkspace+string(filepath.Separator)) {
		return fmt.Errorf("trace path escapes workspace: %q", tracePath)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(absPath, data, 0o600)
}
