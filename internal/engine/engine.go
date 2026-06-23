// Package engine is the autonomous cycle orchestrator — the "brain" that knows
// where the cycle is, what to run next, and what to skip. It walks a
// config.Cycle in board order via Cycle.NextRunnable, resolves each step's model
// through config.Resolver, evaluates AutoSkip sensors, drives the runner, and
// publishes live status onto the Bus. It owns the pending->in_progress->done/
// failed transitions, so step status is correct regardless of how the backend
// streams.
package engine

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

// ErrBusy is returned when a run is requested while one is already active.
var ErrBusy = errors.New("engine: a run is already in progress")

// RunState is the snapshot the GUI shows in its toolbar.
type RunState struct {
	State       string `json:"state"` // idle | running | paused
	RunID       string `json:"run_id,omitempty"`
	CurrentStep string `json:"current_step,omitempty"`
	Cycle       string `json:"cycle,omitempty"`
	Mode        string `json:"mode,omitempty"` // step | cycle
}

// Engine orchestrates runs over a single shared working tree (steps are
// strictly sequential in the MVP).
type Engine struct {
	cfg       *config.Config
	res       *config.Resolver
	run       runner.Runner
	bus       *Bus
	saveCycle func(*config.Cycle) error

	mu       sync.Mutex
	running  bool
	paused   bool // pause = stop gracefully after the current step
	cancelFn context.CancelFunc
	runID    string
	curStep  string
	mode     string
}

// New builds an engine. It constructs even when the runner is unavailable; only
// starting a run then fails, keeping the board usable read-only.
func New(cfg *config.Config, res *config.Resolver, run runner.Runner, bus *Bus) *Engine {
	return &Engine{cfg: cfg, res: res, run: run, bus: bus, saveCycle: config.SaveCycle}
}

// Bus exposes the event bus for the SSE handler.
func (e *Engine) Bus() *Bus { return e.bus }

// Available reports backend availability for the doctor/run surface.
func (e *Engine) Available(ctx context.Context) (bool, runner.Info) {
	return e.run.Available(ctx)
}

// State returns the current run snapshot.
func (e *Engine) State() RunState {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := "idle"
	if e.running {
		st = "running"
		if e.paused {
			st = "paused"
		}
	}
	return RunState{State: st, RunID: e.runID, CurrentStep: e.curStep, Cycle: e.cfg.Defaults.Cycle, Mode: e.mode}
}

// StartStep runs exactly one step (the GUI "Run only this step"). Non-blocking:
// it kicks a goroutine and returns the runID.
func (e *Engine) StartStep(stepID string) (string, error) {
	return e.start("step", func(ctx context.Context, runID string) {
		cycle, err := config.LoadCycle(e.cfg.Defaults.Cycle)
		if err != nil {
			e.bus.Publish(Event{Type: "error", RunID: runID, Line: "load cycle: " + err.Error()})
			return
		}
		_, step := cycle.FindStep(stepID)
		if step == nil {
			e.bus.Publish(Event{Type: "error", RunID: runID, Line: "step not found: " + stepID})
			return
		}
		if !step.Enabled {
			e.bus.Publish(Event{Type: "log", RunID: runID, StepID: stepID, Kind: "log", Line: "step is disabled"})
			return
		}
		e.setCurStep(stepID)
		e.execStep(ctx, cycle, step, runID)
	})
}

// StartCycle runs the whole cycle autonomously from where it left off: it walks
// NextRunnable, executing steps until the cycle completes, halts on a problem,
// is paused, or is cancelled. This is the "run a Cycle" entry point.
func (e *Engine) StartCycle() (string, error) {
	return e.start("cycle", func(ctx context.Context, runID string) {
		cycle, err := config.LoadCycle(e.cfg.Defaults.Cycle)
		if err != nil {
			e.bus.Publish(Event{Type: "error", RunID: runID, Line: "load cycle: " + err.Error()})
			return
		}
		if n := cycle.ResetStuck(); n > 0 {
			if err := e.saveCycle(cycle); err != nil {
				e.bus.Publish(Event{Type: "error", RunID: runID, Line: "persist reset: " + err.Error()})
				return
			}
		}
		for {
			if ctx.Err() != nil || e.isPaused() {
				return
			}
			pi, si, ok := cycle.NextRunnable()
			if !ok {
				e.bus.Publish(Event{Type: "log", RunID: runID, Kind: "log", Line: "cycle complete or halted"})
				return
			}
			step := &cycle.Phases[pi].Steps[si]
			e.setCurStep(step.ID)
			status := e.execStep(ctx, cycle, step, runID)
			// A failed/blocked/needs_review step halts the autonomous walk so we
			// never run past a problem (NextRunnable would return ok=false anyway).
			if status != config.StatusDone && status != config.StatusSkipped {
				e.bus.Publish(Event{Type: "log", RunID: runID, StepID: step.ID, Kind: "log",
					Line: "halting cycle at " + step.ID + " (" + string(status) + ")"})
				return
			}
		}
	})
}

// Cancel hard-stops the active run (cancels the step context).
func (e *Engine) Cancel() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancelFn != nil {
		e.cancelFn()
	}
}

// Pause requests a graceful stop after the current step; Resume re-enters the
// autonomous walk from where it left off.
func (e *Engine) Pause() {
	e.mu.Lock()
	e.paused = true
	e.mu.Unlock()
	e.publishState()
}

// Resume continues an autonomous cycle after a pause.
func (e *Engine) Resume() (string, error) {
	e.mu.Lock()
	wasPaused := e.paused
	e.mu.Unlock()
	if wasPaused {
		e.mu.Lock()
		e.paused = false
		e.mu.Unlock()
	}
	return e.StartCycle()
}

// ---------------------------------------------------------------------------
// internals
// ---------------------------------------------------------------------------

func (e *Engine) start(mode string, body func(ctx context.Context, runID string)) (string, error) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return "", ErrBusy
	}
	runID := "run_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	ctx, cancel := context.WithCancel(context.Background())
	e.running = true
	e.paused = false
	e.cancelFn = cancel
	e.runID = runID
	e.mode = mode
	e.curStep = ""
	e.mu.Unlock()

	e.publishState()

	go func() {
		defer func() {
			cancel()
			e.mu.Lock()
			e.running = false
			e.cancelFn = nil
			e.curStep = ""
			e.mu.Unlock()
			e.publishState()
		}()
		body(ctx, runID)
	}()

	return runID, nil
}

// execStep runs one step: auto-skip probe, then the resolved model with its
// fallback chain, updating status and streaming events. Returns the terminal
// status (done/skipped/failed). The step pointer must belong to cycle.
func (e *Engine) execStep(ctx context.Context, cycle *config.Cycle, step *config.Step, runID string) config.StepStatus {
	dir := e.workingDir()

	// 1) AutoSkip sensor.
	if skip, reason, serr := EvalAutoSkip(ctx, step.AutoSkip, dir); serr != nil {
		e.log(runID, step.ID, "log", "sensor "+step.AutoSkip.Sensor+" failed ("+serr.Error()+") — running step")
	} else if skip {
		e.setStatus(cycle, step, config.StatusSkipped, runID)
		e.log(runID, step.ID, "log", "auto-skip: "+reason)
		return config.StatusSkipped
	}

	// 2) Mark running.
	if err := e.setStatus(cycle, step, config.StatusInProgress, runID); err != nil {
		return config.StatusFailed
	}

	// 3) Resolve model (override > category > default).
	spec, rm, err := e.res.BuildRunSpec(*step, dir)
	if err != nil {
		e.log(runID, step.ID, "error", "resolve model: "+err.Error())
		e.setStatus(cycle, step, config.StatusFailed, runID)
		return config.StatusFailed
	}
	msg := composeMessage(*step)
	e.log(runID, step.ID, "log", "▶ "+step.Name+"  ["+spec.Model+display(spec.Variant)+"]  via "+rm.Source)

	// 4) Run with the fallback chain.
	for {
		req := runner.StepRequest{
			RunID:      runID,
			StepID:     step.ID,
			Skill:      step.Skill,
			Agent:      spec.Agent,
			Model:      spec.Model,
			Variant:    spec.Variant,
			Message:    msg,
			WorkingDir: dir,
			SkipPerms:  e.cfg.Runner.SkipPermissions,
		}
		ch, rerr := e.run.RunStep(ctx, req)
		if rerr != nil {
			e.log(runID, step.ID, "error", "runner: "+rerr.Error())
			e.setStatus(cycle, step, config.StatusFailed, runID)
			return config.StatusFailed
		}

		var doneErr string
		gotDone := false
		for ev := range ch {
			if ev.Kind == runner.EventDone {
				gotDone = true
				doneErr = ev.Err
				continue
			}
			line := firstNonEmpty(ev.Text, ev.Err)
			if line != "" || ev.Kind != runner.EventLog {
				e.log(runID, step.ID, string(ev.Kind), line)
			}
		}
		if !gotDone && doneErr == "" {
			doneErr = "run ended without completion (cancelled?)"
		}

		if doneErr == "" {
			e.setStatus(cycle, step, config.StatusDone, runID)
			e.log(runID, step.ID, "log", "✓ "+step.Name)
			return config.StatusDone
		}

		// Failed. Do not retry on engine-level cancel/timeout.
		if ctx.Err() != nil {
			e.setStatus(cycle, step, config.StatusPending, runID)
			e.log(runID, step.ID, "log", "cancelled — step reset to pending")
			return config.StatusPending
		}
		next, ok := config.NextAttempt(spec, rm)
		if !ok {
			e.setStatus(cycle, step, config.StatusFailed, runID)
			e.log(runID, step.ID, "error", "all models exhausted — "+doneErr)
			return config.StatusFailed
		}
		e.log(runID, step.ID, "log", "model "+spec.Model+" failed ("+doneErr+") — retrying with "+next.Model)
		spec = next
	}
}

// composeMessage builds the prompt that triggers a step's skill. opencode skills
// are mentioned with a leading "$"; the prompt suffix (if any) is appended. A
// step without a skill sends just its suffix as a free prompt.
func composeMessage(s config.Step) string {
	suffix := strings.TrimSpace(s.PromptSuffix)
	if s.Skill != "" {
		msg := "Run the $" + s.Skill + " skill now."
		if suffix != "" {
			msg += "\n\n" + suffix
		}
		return msg
	}
	return suffix
}

func (e *Engine) setStatus(cycle *config.Cycle, step *config.Step, to config.StepStatus, runID string) error {
	if !canTransition(step.Status, to) {
		e.log(runID, step.ID, "log", "warning: irregular transition "+string(step.Status.Effective())+"->"+string(to))
	}
	step.Status = to
	if err := e.saveCycle(cycle); err != nil {
		e.bus.Publish(Event{Type: "error", RunID: runID, StepID: step.ID, Line: "persist: " + err.Error()})
		return err
	}
	e.bus.Publish(Event{Type: "step_status", RunID: runID, StepID: step.ID, Status: string(to)})
	return nil
}

func (e *Engine) log(runID, stepID, kind, line string) {
	if line == "" && kind == "log" {
		return
	}
	e.bus.Publish(Event{Type: "log", RunID: runID, StepID: stepID, Kind: kind, Line: line})
}

func (e *Engine) workingDir() string {
	if e.cfg.Runner.WorkingDir != "" {
		return e.cfg.Runner.WorkingDir
	}
	return "" // current process dir
}

func (e *Engine) setCurStep(id string) {
	e.mu.Lock()
	e.curStep = id
	e.mu.Unlock()
	e.publishState()
}

func (e *Engine) isPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.paused
}

func (e *Engine) publishState() {
	st := e.State()
	e.bus.Publish(Event{Type: "run_state", RunID: st.RunID, StepID: st.CurrentStep, State: st.State})
}

func display(variant string) string {
	if variant == "" {
		return ""
	}
	return " " + variant
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
