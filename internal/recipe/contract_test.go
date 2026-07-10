package recipe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

// mockRunner implements runner.Runner for testing.
type mockRunner struct {
	available bool
	info      runner.Info
	runStep   func(context.Context, runner.StepRequest) (<-chan runner.RunEvent, error)
}

func (r *mockRunner) Name() string                                    { return "mock" }
func (r *mockRunner) Available(_ context.Context) (bool, runner.Info) { return r.available, r.info }
func (r *mockRunner) RunStep(ctx context.Context, req runner.StepRequest) (<-chan runner.RunEvent, error) {
	if r.runStep != nil {
		return r.runStep(ctx, req)
	}
	ch := make(chan runner.RunEvent, 2)
	ch <- runner.RunEvent{Kind: runner.EventDone, Text: "completed"}
	close(ch)
	return ch, nil
}

// newSuccessRunner returns a mock that completes successfully with a single event.
func newSuccessRunner() *mockRunner {
	return &mockRunner{
		available: true,
		info:      runner.Info{Name: "mock", Version: "1.0"},
	}
}

// newFailingRunner returns a mock that reports a failed run.
func newFailingRunner() *mockRunner {
	return &mockRunner{
		available: true,
		info:      runner.Info{Name: "mock", Version: "1.0"},
		runStep: func(_ context.Context, _ runner.StepRequest) (<-chan runner.RunEvent, error) {
			ch := make(chan runner.RunEvent, 2)
			ch <- runner.RunEvent{Kind: runner.EventError, Err: "something went wrong"}
			ch <- runner.RunEvent{Kind: runner.EventDone, Err: "something went wrong"}
			close(ch)
			return ch, nil
		},
	}
}

// newUnavailableRunner returns a mock that is not available.
func newUnavailableRunner() *mockRunner {
	return &mockRunner{
		available: false,
		info:      runner.Info{Name: "mock", Detail: "not installed"},
	}
}

// newErrorRunner returns a mock whose RunStep itself errors.
func newErrorRunner() *mockRunner {
	return &mockRunner{
		available: true,
		info:      runner.Info{Name: "mock", Version: "1.0"},
		runStep: func(_ context.Context, _ runner.StepRequest) (<-chan runner.RunEvent, error) {
			return nil, errors.New("process failed")
		},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     RecipeRequest
		wantErr error
	}{
		{
			name:    "missing prompt",
			req:     RecipeRequest{SchemaVersion: "1", Workspace: "/tmp"},
			wantErr: ErrMissingPrompt,
		},
		{
			name:    "missing workspace",
			req:     RecipeRequest{SchemaVersion: "1", Prompt: "do something"},
			wantErr: ErrMissingWorkspace,
		},
		{
			name:    "wrong schema version",
			req:     RecipeRequest{SchemaVersion: "99", Prompt: "x", Workspace: "/tmp"},
			wantErr: ErrInvalidSchema,
		},
		{
			name:    "missing schema version defaults to 1",
			req:     RecipeRequest{Prompt: "x", Workspace: "/tmp"},
			wantErr: nil,
		},
		{
			name: "valid request with defaults",
			req:  RecipeRequest{SchemaVersion: "1", Prompt: "x", Workspace: "/tmp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
			// Check defaults are applied.
			if tt.wantErr == nil && tt.req.WriteCap == "" {
				t.Error("WriteCap should default to workspace")
			}
			if tt.wantErr == nil && tt.req.SchemaVersion == "" {
				t.Error("SchemaVersion should default to 1")
			}
		})
	}
}

func TestServiceRunValidation(t *testing.T) {
	svc := NewService(newSuccessRunner())
	tests := []struct {
		name    string
		req     RecipeRequest
		wantErr error
	}{
		{"missing prompt", RecipeRequest{Workspace: "/tmp"}, ErrMissingPrompt},
		{"missing workspace", RecipeRequest{Prompt: "do something"}, ErrMissingWorkspace},
		{"wrong schema", RecipeRequest{SchemaVersion: "99", Prompt: "x", Workspace: "/tmp"}, ErrInvalidSchema},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Run(context.Background(), tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Run() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceRunBackendUnavailable(t *testing.T) {
	svc := NewService(newUnavailableRunner())
	_, err := svc.Run(context.Background(), RecipeRequest{
		SchemaVersion: "1",
		Prompt:        "do something",
		Workspace:     t.TempDir(),
	})
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Errorf("Run() error = %v, want %v", err, ErrBackendUnavailable)
	}
}

func TestServiceRunSuccess(t *testing.T) {
	svc := NewService(newSuccessRunner())
	result, err := svc.Run(context.Background(), RecipeRequest{
		SchemaVersion: "1",
		Prompt:        "write a test file",
		Workspace:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if result.Status != "done" {
		t.Errorf("Status = %q, want %q", result.Status, "done")
	}
	if result.RunID == "" {
		t.Error("RunID should be non-empty")
	}
	if result.Backend != "mock" {
		t.Errorf("Backend = %q, want %q", result.Backend, "mock")
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}
	if result.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", result.SchemaVersion, "1")
	}
}

func TestServiceRunFailingBackend(t *testing.T) {
	svc := NewService(newFailingRunner())
	result, err := svc.Run(context.Background(), RecipeRequest{
		SchemaVersion: "1",
		Prompt:        "this will fail",
		Workspace:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("Status = %q, want %q", result.Status, "failed")
	}
	if result.Error == "" {
		t.Error("Error should be non-empty for failed run")
	}
}

func TestServiceRunProcessError(t *testing.T) {
	svc := NewService(newErrorRunner())
	result, err := svc.Run(context.Background(), RecipeRequest{
		SchemaVersion: "1",
		Prompt:        "this will error",
		Workspace:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("Status = %q, want %q", result.Status, "failed")
	}
}

func TestServiceRunTraceCollection(t *testing.T) {
	ch := make(chan runner.RunEvent, 5)
	ch <- runner.RunEvent{Kind: runner.EventStart, Text: "started"}
	ch <- runner.RunEvent{Kind: runner.EventTool, Text: "read_file"}
	ch <- runner.RunEvent{Kind: runner.EventLog, Text: "processing"}
	ch <- runner.RunEvent{Kind: runner.EventDone, Text: "completed"}
	close(ch)

	run := &mockRunner{
		available: true,
		info:      runner.Info{Name: "mock"},
		runStep: func(_ context.Context, _ runner.StepRequest) (<-chan runner.RunEvent, error) {
			return ch, nil
		},
	}

	svc := NewService(run)
	result, err := svc.Run(context.Background(), RecipeRequest{
		SchemaVersion: "1",
		Prompt:        "do stuff",
		Workspace:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if len(result.Trace) != 4 {
		t.Errorf("Trace has %d events, want 4", len(result.Trace))
	}
	if result.Trace[0].Kind != runner.EventStart {
		t.Errorf("Trace[0].Kind = %q, want %q", result.Trace[0].Kind, runner.EventStart)
	}
}

func TestServiceRunToolAllowListEcho(t *testing.T) {
	svc := NewService(newSuccessRunner())
	tools := []string{"read_file", "write_file", "git_status"}
	result, err := svc.Run(context.Background(), RecipeRequest{
		SchemaVersion: "1",
		Prompt:        "do stuff",
		Workspace:     t.TempDir(),
		ToolAllowList: tools,
	})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if len(result.ToolAllowList) != 3 {
		t.Errorf("ToolAllowList has %d items, want 3", len(result.ToolAllowList))
	}
}

func TestServiceRunWriteCapEcho(t *testing.T) {
	svc := NewService(newSuccessRunner())
	result, err := svc.Run(context.Background(), RecipeRequest{
		SchemaVersion: "1",
		Prompt:        "do stuff",
		Workspace:     t.TempDir(),
		WriteCap:      WriteCapNone,
	})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if result.WriteCap != WriteCapNone {
		t.Errorf("WriteCap = %q, want %q", result.WriteCap, WriteCapNone)
	}
}

func TestServiceRunCancellation(t *testing.T) {
	run := &mockRunner{
		available: true,
		info:      runner.Info{Name: "mock"},
		runStep: func(ctx context.Context, _ runner.StepRequest) (<-chan runner.RunEvent, error) {
			ch := make(chan runner.RunEvent, 1)
			go func() {
				defer close(ch)
				// Wait for context cancellation.
				<-ctx.Done()
				ch <- runner.RunEvent{Kind: runner.EventDone, Err: "cancelled"}
			}()
			return ch, nil
		},
	}

	svc := NewService(run)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := svc.Run(ctx, RecipeRequest{
		SchemaVersion: "1",
		Prompt:        "long task",
		Workspace:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	// The context was cancelled before the runner could emit a done event.
	if result.Status != "failed" {
		t.Errorf("Status = %q, want %q (cancelled run should fail)", result.Status, "failed")
	}
}

func TestCollectTraceEmpty(t *testing.T) {
	ch := make(chan runner.RunEvent)
	close(ch)
	trace := collectTrace(context.Background(), ch)
	if len(trace) != 0 {
		t.Errorf("collectTrace on empty channel returned %d events, want 0", len(trace))
	}
}

func TestCollectTraceContextCancelled(t *testing.T) {
	ch := make(chan runner.RunEvent)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	trace := collectTrace(ctx, ch)
	if len(trace) != 0 {
		t.Errorf("collectTrace with cancelled ctx returned %d events, want 0", len(trace))
	}
}

func TestProposedDiff(t *testing.T) {
	tests := []struct {
		name string
		pre  string
		post string
		want string
	}{
		{"no changes", "", "", ""},
		{"clean to dirty", "", "diff --git a/foo", "diff --git a/foo"},
		{"dirty to dirtier", "diff --git a/old", "diff --git a/new", "diff --git a/new"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proposedDiff(tt.pre, tt.post)
			if got != tt.want {
				t.Errorf("proposedDiff() = %q, want %q", got, tt.want)
			}
		})
	}
}
