package engine

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

type countingRunner struct {
	calls int
}

func (r *countingRunner) Name() string { return "test" }

func (r *countingRunner) Available(context.Context) (bool, runner.Info) {
	return true, runner.Info{Name: "test"}
}

func (r *countingRunner) RunStep(context.Context, runner.StepRequest) (<-chan runner.RunEvent, error) {
	r.calls++
	return nil, errors.New("runner should not start")
}

func (r *countingRunner) successfulRun(context.Context, runner.StepRequest) (<-chan runner.RunEvent, error) {
	r.calls++
	ch := make(chan runner.RunEvent, 1)
	ch <- runner.RunEvent{Kind: runner.EventDone}
	close(ch)
	return ch, nil
}

type cancellingRunner struct {
	started chan struct{}
}

func (r *cancellingRunner) Name() string { return "test" }

func (r *cancellingRunner) Available(context.Context) (bool, runner.Info) {
	return true, runner.Info{Name: "test"}
}

func (r *cancellingRunner) RunStep(ctx context.Context, _ runner.StepRequest) (<-chan runner.RunEvent, error) {
	ch := make(chan runner.RunEvent, 1)
	close(r.started)
	go func() {
		defer close(ch)
		<-ctx.Done()
		ch <- runner.RunEvent{Kind: runner.EventDone, Err: "cancelled"}
	}()
	return ch, nil
}

func TestCancelResetsActiveStepToPending(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "cancel-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{{ID: "step", Enabled: true}},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &cancellingRunner{started: make(chan struct{})}
	eng := New(cfg, config.NewResolver(cfg), run, NewBus())
	if _, err := eng.StartCycle(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-run.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	eng.Cancel()

	deadline := time.Now().Add(time.Second)
	for eng.State().State != "idle" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if eng.State().State != "idle" {
		t.Fatal("engine did not stop after cancellation")
	}
	stored, err := config.LoadCycle(cfg.Defaults.Cycle)
	if err != nil {
		t.Fatal(err)
	}
	_, step := stored.FindStep("step")
	if step == nil || step.Status != config.StatusPending {
		t.Fatalf("cancelled step status = %#v, want pending", step)
	}
}

func TestStartCycleDoesNotRunStepWhenInProgressStatusCannotPersist(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "persistence-test"
	if err := config.SaveCycle(&config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{{ID: "step", Enabled: true}},
	}}}); err != nil {
		t.Fatal(err)
	}

	run := &countingRunner{}
	eng := New(cfg, config.NewResolver(cfg), run, NewBus())
	eng.saveCycle = func(*config.Cycle) error { return errors.New("disk unavailable") }
	if _, err := eng.StartCycle(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for eng.State().State != "idle" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if run.calls != 0 {
		t.Fatalf("runner started %d times after persistence failure", run.calls)
	}
}

func TestStartCycleStopsWhenDoneStatusCannotPersist(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "terminal-persistence-test"
	if err := config.SaveCycle(&config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{{ID: "one", Enabled: true}, {ID: "two", Enabled: true}},
	}}}); err != nil {
		t.Fatal(err)
	}

	run := &countingRunner{}
	eng := New(cfg, config.NewResolver(cfg), runnerFunc(run.successfulRun), NewBus())
	saves := 0
	eng.saveCycle = func(*config.Cycle) error {
		saves++
		if saves == 2 {
			return errors.New("disk unavailable")
		}
		return nil
	}
	if _, err := eng.StartCycle(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for eng.State().State != "idle" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if run.calls != 1 {
		t.Fatalf("runner started %d times after terminal persistence failure, want 1", run.calls)
	}
}

type runnerFunc func(context.Context, runner.StepRequest) (<-chan runner.RunEvent, error)

func (f runnerFunc) Name() string { return "test" }

func (f runnerFunc) Available(context.Context) (bool, runner.Info) {
	return true, runner.Info{Name: "test"}
}

func (f runnerFunc) RunStep(ctx context.Context, req runner.StepRequest) (<-chan runner.RunEvent, error) {
	return f(ctx, req)
}

func TestBackendOverrideInstantiatesNewRunner(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "override-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{{
			ID:              "step",
			Enabled:         true,
			BackendOverride: "api",
		}},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &countingRunner{}
	eng := New(cfg, config.NewResolver(cfg), run, NewBus())

	if _, err := eng.StartCycle(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for eng.State().State != "idle" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	if run.calls != 0 {
		t.Fatalf("expected default runner to be bypassed, but it was called %d times", run.calls)
	}

	stored, err := config.LoadCycle(cfg.Defaults.Cycle)
	if err != nil {
		t.Fatal(err)
	}
	_, step := stored.FindStep("step")
	if step == nil || step.Status != config.StatusFailed {
		t.Fatalf("expected step to fail due to unavailable API runner, but status is %v", step.Status)
	}
}

// ---------------------------------------------------------------------------
// run-control API: StartStep, Pause/Resume, CleanupWorkspace,
// WaitForRunDone, LatestRunSummary
// ---------------------------------------------------------------------------

// gatedRunner blocks each step until release is closed (or the step context
// is cancelled), letting tests hold a run mid-cycle deterministically.
type gatedRunner struct {
	calls   atomic.Int32
	started chan struct{} // buffered; one token per RunStep call
	release chan struct{} // closing it lets the current step finish
}

func (r *gatedRunner) Name() string { return "test" }

func (r *gatedRunner) Available(context.Context) (bool, runner.Info) {
	return true, runner.Info{Name: "test"}
}

func (r *gatedRunner) RunStep(ctx context.Context, _ runner.StepRequest) (<-chan runner.RunEvent, error) {
	r.calls.Add(1)
	select {
	case r.started <- struct{}{}:
	default:
	}
	ch := make(chan runner.RunEvent, 1)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			ch <- runner.RunEvent{Kind: runner.EventDone, Err: "cancelled"}
		case <-r.release:
			ch <- runner.RunEvent{Kind: runner.EventDone}
		}
	}()
	return ch, nil
}

// waitForIdle polls State until the engine reports idle or the deadline hits.
func waitForIdle(t *testing.T, eng *Engine) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for eng.State().State != "idle" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if eng.State().State != "idle" {
		t.Fatal("engine did not return to idle within 5s")
	}
}

func TestStartStepRunsExactlyOneStep(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "start-step-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{
			{ID: "one", Enabled: true},
			{ID: "two", Enabled: true},
		},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &countingRunner{}
	eng := New(cfg, config.NewResolver(cfg), runnerFunc(run.successfulRun), NewBus())

	runID, err := eng.StartStep("one")
	if err != nil {
		t.Fatal(err)
	}
	if runID == "" {
		t.Fatal("StartStep returned an empty runID")
	}
	waitForIdle(t, eng)
	if run.calls != 1 {
		t.Fatalf("runner executed %d steps, want 1 (a single step run must halt)", run.calls)
	}

	stored, err := config.LoadCycle(cfg.Defaults.Cycle)
	if err != nil {
		t.Fatal(err)
	}
	_, one := stored.FindStep("one")
	if one == nil || one.Status != config.StatusDone {
		t.Fatalf("step 'one' status = %v, want done", one.Status)
	}
	_, two := stored.FindStep("two")
	if two == nil || two.Status == config.StatusDone {
		t.Fatalf("step 'two' must not have run, status = %v", two.Status)
	}
}

func TestStartStepDisabledStepHaltsWithoutRunning(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "start-step-disabled-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{
			{ID: "one", Enabled: false},
			{ID: "two", Enabled: true},
		},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &countingRunner{}
	eng := New(cfg, config.NewResolver(cfg), run, NewBus())

	if _, err := eng.StartStep("one"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, eng)
	if run.calls != 0 {
		t.Fatalf("runner executed %d steps for a disabled step, want 0", run.calls)
	}

	stored, err := config.LoadCycle(cfg.Defaults.Cycle)
	if err != nil {
		t.Fatal(err)
	}
	_, one := stored.FindStep("one")
	if one == nil || one.Status == config.StatusDone {
		t.Fatalf("disabled step 'one' status = %v, want untouched", one.Status)
	}
}

func TestStartStepMissingStepHaltsWithoutRunning(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "start-step-missing-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{{ID: "one", Enabled: true}},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &countingRunner{}
	eng := New(cfg, config.NewResolver(cfg), run, NewBus())

	if _, err := eng.StartStep("ghost"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, eng)
	if run.calls != 0 {
		t.Fatalf("runner executed %d steps for a missing step, want 0", run.calls)
	}
	if st := eng.State(); st.State != "idle" {
		t.Fatalf("state = %q, want idle", st.State)
	}
}

func TestPauseResumeRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "pause-resume-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{
			{ID: "one", Enabled: true},
			{ID: "two", Enabled: true},
		},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &gatedRunner{started: make(chan struct{}, 1), release: make(chan struct{})}
	eng := New(cfg, config.NewResolver(cfg), run, NewBus())

	// Run 1: hold the walk inside step 'one', then pause.
	if _, err := eng.StartCycle(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-run.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	eng.Pause()
	if st := eng.State(); st.State != "paused" {
		t.Fatalf("state after Pause = %q, want paused", st.State)
	}

	// Releasing the gate lets step 'one' finish; the walk must halt at the
	// pause instead of continuing to step 'two'.
	close(run.release)
	eng.WaitForRunDone()
	if st := eng.State(); st.State != "idle" {
		t.Fatalf("state after pause halt = %q, want idle", st.State)
	}
	if run.calls.Load() != 1 {
		t.Fatalf("run 1 executed %d steps, want 1 (paused walk must halt)", run.calls.Load())
	}

	// Resume re-enters the autonomous walk and completes the remaining step.
	if _, err := eng.Resume(); err != nil {
		t.Fatal(err)
	}
	if st := eng.State(); st.State != "running" {
		t.Fatalf("state after Resume = %q, want running", st.State)
	}
	select {
	case <-run.started:
	case <-time.After(time.Second):
		t.Fatal("resumed run did not start a step")
	}
	eng.WaitForRunDone()
	if st := eng.State(); st.State != "idle" {
		t.Fatalf("state after resumed run = %q, want idle", st.State)
	}
	if run.calls.Load() != 2 {
		t.Fatalf("resumed run executed %d steps total, want 2", run.calls.Load())
	}

	stored, err := config.LoadCycle(cfg.Defaults.Cycle)
	if err != nil {
		t.Fatal(err)
	}
	_, one := stored.FindStep("one")
	if one == nil || one.Status != config.StatusDone {
		t.Fatalf("step 'one' status = %v, want done", one.Status)
	}
	_, two := stored.FindStep("two")
	if two == nil || two.Status != config.StatusDone {
		t.Fatalf("step 'two' status = %v, want done", two.Status)
	}
}

func TestCleanupWorkspaceIdempotent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	eng := New(cfg, config.NewResolver(cfg), &countingRunner{}, NewBus())

	cleanupCalls := 0
	eng.setWorkspaceInfo(&WorkspaceInfo{
		Path:      t.TempDir(),
		Mode:      "isolated",
		Branch:    "workspace/cycle-cleanup-test",
		CleanupFn: func() { cleanupCalls++ },
	})

	eng.CleanupWorkspace()
	if eng.WorkspaceInfo() != nil {
		t.Fatal("workspace info still set after first CleanupWorkspace")
	}
	eng.CleanupWorkspace() // second call must be a no-op
	if cleanupCalls != 1 {
		t.Fatalf("cleanup callback invoked %d times, want 1", cleanupCalls)
	}
}

// TestCleanupWorkspaceConcurrentWithStartCycle is the regression test for the
// #141 data race: start() previously read e.wsInfo after dropping the mutex
// while CleanupWorkspace nils it under the mutex. The hammer goroutine nils
// and re-reads the workspace concurrently with repeated StartCycle calls, so
// the interleaving is exercised; the test must pass under -race.
func TestCleanupWorkspaceConcurrentWithStartCycle(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "cleanup-concurrent-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{{ID: "step", Enabled: true}},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &countingRunner{}
	eng := New(cfg, config.NewResolver(cfg), runnerFunc(run.successfulRun), NewBus())
	// Prime e.wsInfo so start() has a non-nil snapshot to read; a real run
	// sets it via setWorkspaceInfo and only CleanupWorkspace nils it.
	eng.setWorkspaceInfo(&WorkspaceInfo{Path: t.TempDir(), Mode: "shared"})

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				eng.CleanupWorkspace()
				eng.WorkspaceInfo()
			}
		}
	}()

	for i := 0; i < 100; i++ {
		eng.setWorkspaceInfo(&WorkspaceInfo{Path: t.TempDir(), Mode: "shared"})
		if _, err := eng.StartCycle(); err != nil && err != ErrBusy {
			t.Fatal(err)
		}
		eng.WaitForRunDone()
	}
	close(stop)
	wg.Wait()

	eng.CleanupWorkspace()
	if eng.WorkspaceInfo() != nil {
		t.Fatal("workspace info not cleared after final cleanup")
	}
}

func TestWaitForRunDoneAndLatestRunSummary(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "wait-run-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{{ID: "step", Enabled: true}},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	run := &gatedRunner{started: make(chan struct{}, 1), release: make(chan struct{})}
	eng := New(cfg, config.NewResolver(cfg), run, NewBus())

	// No summary exists before the first run.
	if s, err := eng.LatestRunSummary(); err != nil || s != nil {
		t.Fatalf("pre-run LatestRunSummary = %v, %v; want nil, nil", s, err)
	}

	runID, err := eng.StartCycle()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-run.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}

	// WaitForRunDone must block while the run is still active.
	done := make(chan struct{})
	go func() {
		eng.WaitForRunDone()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("WaitForRunDone returned while the run was still gated")
	case <-time.After(100 * time.Millisecond):
		// expected: still blocked
	}

	close(run.release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForRunDone did not return after run completion")
	}
	if st := eng.State(); st.State != "idle" {
		t.Fatalf("state after run completion = %q, want idle", st.State)
	}

	// LatestRunSummary returns the persisted summary for the finished run.
	summary, err := eng.LatestRunSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary == nil {
		t.Fatal("LatestRunSummary returned nil after a finished run")
	}
	if summary.RunID != runID {
		t.Fatalf("summary run id = %q, want %q", summary.RunID, runID)
	}
	if summary.Mode != "cycle" || summary.WorkspaceMode != "shared" {
		t.Fatalf("summary mode = %q/%q, want cycle/shared", summary.Mode, summary.WorkspaceMode)
	}
	if summary.Status != "complete" {
		t.Fatalf("summary status = %q, want complete", summary.Status)
	}
	if len(summary.Steps) != 1 || summary.Steps[0].StepID != "step" || summary.Steps[0].FinalStatus != "done" {
		t.Fatalf("summary steps = %+v, want one done 'step'", summary.Steps)
	}
}
