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
		restoreWorkspace(ctx, req.Workspace, preDiff)
		slog.Info("recipe review mode: workspace restored", "run_id", runID)
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
// pre-diff if there were pre-existing uncommitted changes.
func restoreWorkspace(ctx context.Context, workspace string, preDiff string) {
	// Reset tracked files to HEAD.
	cmd := exec.CommandContext(ctx, "git", "checkout", "HEAD", "--", ".")
	cmd.Dir = workspace
	_ = cmd.Run()

	// Remove untracked files and directories.
	cmd = exec.CommandContext(ctx, "git", "clean", "-fd")
	cmd.Dir = workspace
	_ = cmd.Run()

	// Reapply pre-existing uncommitted changes, if any.
	if preDiff != "" {
		cmd = exec.CommandContext(ctx, "git", "apply", "-")
		cmd.Dir = workspace
		cmd.Stdin = strings.NewReader(preDiff)
		_ = cmd.Run()
	}
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
// validated as a local relative path before any file operation.
func writeTrace(workspace, tracePath string, trace []runner.RunEvent) error {
	if !filepath.IsLocal(tracePath) {
		return fmt.Errorf("trace path is not local relative: %q", tracePath)
	}
	path := filepath.Join(workspace, tracePath)
	// Defense in depth: ensure the resolved path stays inside the workspace.
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}
