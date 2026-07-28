package engine

import (
	"context"
	"testing"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

func TestEvalRequiresReviewNilRule(t *testing.T) {
	step := &config.Step{Category: "release"}
	if review, reason := EvalRequiresReview(nil, step); review || reason != "" {
		t.Fatalf("nil rule: got (%v, %q), want (false, \"\")", review, reason)
	}
}

func TestEvalRequiresReviewEmptyWhen(t *testing.T) {
	step := &config.Step{Category: "release"}
	if review, _ := EvalRequiresReview(&config.RequiresReview{When: "  "}, step); review {
		t.Fatal("empty when must not match")
	}
}

func TestEvalRequiresReviewCategoryMatch(t *testing.T) {
	step := &config.Step{Category: "release"}
	review, reason := EvalRequiresReview(&config.RequiresReview{When: "category == release"}, step)
	if !review || reason == "" {
		t.Fatalf("category match: got (%v, %q), want (true, reason)", review, reason)
	}
}

func TestEvalRequiresReviewCategoryQuoted(t *testing.T) {
	step := &config.Step{Category: "release"}
	review, _ := EvalRequiresReview(&config.RequiresReview{When: `category == "release"`}, step)
	if !review {
		t.Fatal("quoted value must match")
	}
}

func TestEvalRequiresReviewCategoryNoMatch(t *testing.T) {
	step := &config.Step{Category: "implement"}
	if review, _ := EvalRequiresReview(&config.RequiresReview{When: "category == release"}, step); review {
		t.Fatal("non-matching category must not trigger review")
	}
}

func TestEvalRequiresReviewCategoryNotEqual(t *testing.T) {
	if review, _ := EvalRequiresReview(&config.RequiresReview{When: "category != release"}, &config.Step{Category: "implement"}); !review {
		t.Fatal("!= must match a different category")
	}
	if review, _ := EvalRequiresReview(&config.RequiresReview{When: "category != release"}, &config.Step{Category: "release"}); review {
		t.Fatal("!= must not match the same category")
	}
}

func TestEvalRequiresReviewUnknownAttribute(t *testing.T) {
	review, reason := EvalRequiresReview(&config.RequiresReview{When: "skill == release"}, &config.Step{})
	if review || reason == "" {
		t.Fatalf("unknown attribute: got (%v, %q), want (false, explanation)", review, reason)
	}
}

func TestEvalRequiresReviewUnparseable(t *testing.T) {
	review, reason := EvalRequiresReview(&config.RequiresReview{When: "release"}, &config.Step{Category: "release"})
	if review || reason == "" {
		t.Fatalf("unparseable when: got (%v, %q), want (false, explanation)", review, reason)
	}
}

// waitIdle blocks until the engine finishes its run.
func waitIdle(t *testing.T, eng *Engine) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for eng.State().State != "idle" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if eng.State().State != "idle" {
		t.Fatal("engine did not stop")
	}
}

func successRunner(calls *int) runner.Runner {
	return runnerFunc(func(context.Context, runner.StepRequest) (<-chan runner.RunEvent, error) {
		*calls++
		ch := make(chan runner.RunEvent, 1)
		ch <- runner.RunEvent{Kind: runner.EventDone}
		close(ch)
		return ch, nil
	})
}

func TestStartCycleMovesMatchingStepToNeedsReviewAndHalts(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "requires-review-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{
			{ID: "one", Enabled: true, Category: "release",
				RequiresReview: &config.RequiresReview{When: "category == release"}},
			{ID: "two", Enabled: true},
		},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	calls := 0
	eng := New(cfg, config.NewResolver(cfg), successRunner(&calls), NewBus())
	if _, err := eng.StartCycle(); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, eng)

	if calls != 1 {
		t.Fatalf("runner calls = %d, want 1 (cycle must halt at needs_review)", calls)
	}
	stored, err := config.LoadCycle(cfg.Defaults.Cycle)
	if err != nil {
		t.Fatal(err)
	}
	_, one := stored.FindStep("one")
	if one == nil || one.Status != config.StatusNeedsReview {
		t.Fatalf("step one status = %v, want needs_review", one.Status)
	}
	_, two := stored.FindStep("two")
	if two == nil || two.Status.Effective() != config.StatusPending {
		t.Fatalf("step two status = %v, want pending (never ran)", two.Status)
	}
	if _, _, ok := stored.NextRunnable(); ok {
		t.Fatal("NextRunnable must halt on a needs_review step")
	}
}

func TestStartCycleNonMatchingRuleAdvancesNormally(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "requires-review-nomatch-test"
	cycle := &config.Cycle{ID: cfg.Defaults.Cycle, Phases: []config.Phase{{
		ID: "phase", Steps: []config.Step{
			{ID: "one", Enabled: true, Category: "implement",
				RequiresReview: &config.RequiresReview{When: "category == release"}},
			{ID: "two", Enabled: true},
		},
	}}}
	if err := config.SaveCycle(cycle); err != nil {
		t.Fatal(err)
	}

	calls := 0
	eng := New(cfg, config.NewResolver(cfg), successRunner(&calls), NewBus())
	if _, err := eng.StartCycle(); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, eng)

	if calls != 2 {
		t.Fatalf("runner calls = %d, want 2 (no review gate matched)", calls)
	}
	stored, err := config.LoadCycle(cfg.Defaults.Cycle)
	if err != nil {
		t.Fatal(err)
	}
	_, one := stored.FindStep("one")
	if one == nil || one.Status != config.StatusDone {
		t.Fatalf("step one status = %v, want done", one.Status)
	}
}
