package recipe

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

// fakeRunner is a runner.Runner that optionally writes files into the
// workspace during RunStep and then reports success.
type fakeRunner struct {
	// writes maps workspace-relative paths to content written during the run.
	writes map[string]string
}

func (f *fakeRunner) Name() string { return "fake" }

func (f *fakeRunner) Available(context.Context) (bool, runner.Info) {
	return true, runner.Info{Name: "fake"}
}

func (f *fakeRunner) RunStep(_ context.Context, req runner.StepRequest) (<-chan runner.RunEvent, error) {
	for rel, content := range f.writes {
		abs := filepath.Join(req.WorkingDir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return nil, err
		}
	}
	ch := make(chan runner.RunEvent, 2)
	ch <- runner.RunEvent{Kind: runner.EventStart, Text: "started"}
	ch <- runner.RunEvent{Kind: runner.EventDone, Text: "done"}
	close(ch)
	return ch, nil
}

// git runs a git command in dir and fails the test on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// initRepo creates a git repository with one committed file.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "tracked.txt")
	git(t, dir, "commit", "-m", "init")
	return dir
}

// TestRunReviewModeRefusesPreExistingUntracked verifies that a review_mode run
// against a workspace with pre-existing untracked files is refused and the
// files are left untouched.
func TestRunReviewModeRefusesPreExistingUntracked(t *testing.T) {
	dir := initRepo(t)
	scratch := filepath.Join(dir, "scratch.txt")
	if err := os.WriteFile(scratch, []byte("do not delete me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(&fakeRunner{})
	_, err := svc.Run(context.Background(), RecipeRequest{
		Workspace:  dir,
		Prompt:     "do something",
		ReviewMode: true,
	})
	if !errors.Is(err, ErrUnsafeReviewWorkspace) {
		t.Fatalf("expected ErrUnsafeReviewWorkspace, got %v", err)
	}

	// The pre-existing untracked file must survive.
	data, readErr := os.ReadFile(scratch)
	if readErr != nil {
		t.Fatalf("pre-existing untracked file was deleted: %v", readErr)
	}
	if string(data) != "do not delete me\n" {
		t.Fatalf("pre-existing untracked file was modified: %q", data)
	}
}

// TestRunReviewModeRestoresAndReportsOK verifies that a review_mode run on a
// clean git workspace restores tracked files, removes run-created untracked
// files, and reports RestoreStatus "ok".
func TestRunReviewModeRestoresAndReportsOK(t *testing.T) {
	dir := initRepo(t)

	svc := NewService(&fakeRunner{writes: map[string]string{
		"tracked.txt": "modified by run\n",
		"new.txt":     "created by run\n",
	}})
	result, err := svc.Run(context.Background(), RecipeRequest{
		Workspace:  dir,
		Prompt:     "do something",
		ReviewMode: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != "done" {
		t.Fatalf("status = %q, want done", result.Status)
	}
	if result.RestoreStatus != "ok" {
		t.Fatalf("RestoreStatus = %q, want ok (RestoreError %q)", result.RestoreStatus, result.RestoreError)
	}
	if result.RestoreError != "" {
		t.Fatalf("RestoreError = %q, want empty", result.RestoreError)
	}

	// Tracked file restored to the committed content.
	data, err := os.ReadFile(filepath.Join(dir, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original\n" {
		t.Fatalf("tracked.txt = %q, want restored %q", data, "original\n")
	}
	// Run-created untracked file removed by the restore.
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("run-created new.txt should be removed by restore, stat err = %v", err)
	}
}

// TestRunReviewModeRestoreFailureReported verifies that a failing restore is
// reported on the result instead of being silently swallowed.
func TestRunReviewModeRestoreFailureReported(t *testing.T) {
	// A plain directory that is not a git repository: every restore command
	// fails, and the failure must surface on the result.
	dir := t.TempDir()

	svc := NewService(&fakeRunner{})
	result, err := svc.Run(context.Background(), RecipeRequest{
		Workspace:  dir,
		Prompt:     "do something",
		ReviewMode: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.RestoreStatus != "failed" {
		t.Fatalf("RestoreStatus = %q, want failed", result.RestoreStatus)
	}
	if result.RestoreError == "" {
		t.Fatal("RestoreError is empty, want failure detail")
	}
}

// TestRunNonReviewModeLeavesRestoreStatusEmpty verifies that non-review-mode
// runs do not populate the restore fields.
func TestRunNonReviewModeLeavesRestoreStatusEmpty(t *testing.T) {
	dir := initRepo(t)

	svc := NewService(&fakeRunner{})
	result, err := svc.Run(context.Background(), RecipeRequest{
		Workspace: dir,
		Prompt:    "do something",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.RestoreStatus != "" || result.RestoreError != "" {
		t.Fatalf("restore fields populated on non-review run: %q / %q", result.RestoreStatus, result.RestoreError)
	}
}
