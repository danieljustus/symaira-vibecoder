package engine

import (
	"context"
	"errors"
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

