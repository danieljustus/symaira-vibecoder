package config

import "testing"

func step(id string, st StepStatus) Step {
	return Step{ID: id, Name: id, Enabled: true, Status: st}
}

func cyc(steps ...Step) *Cycle {
	return &Cycle{ID: "t", Phases: []Phase{{ID: "p1", Name: "P1", Steps: steps}}}
}

func TestStepStatusEffective(t *testing.T) {
	if StepStatus("").Effective() != StatusPending {
		t.Fatal("empty status must normalize to pending")
	}
	if StatusFailed.IsTerminal() {
		t.Fatal("failed must NOT be terminal (it halts, not passes)")
	}
	if !StatusFailed.IsHalting() {
		t.Fatal("failed must be halting")
	}
}

func TestNextRunnableSkipsTerminalAndStopsAtPending(t *testing.T) {
	c := cyc(step("1.1", StatusDone), step("1.2", StatusSkipped), step("1.3", ""))
	pi, si, ok := c.NextRunnable()
	if !ok || c.Phases[pi].Steps[si].ID != "1.3" {
		t.Fatalf("want 1.3 runnable, got pi=%d si=%d ok=%v", pi, si, ok)
	}
}

func TestNextRunnableHaltsOnFailed(t *testing.T) {
	// A failed step must HALT the walk, not be skipped or re-selected forever.
	c := cyc(step("1.1", StatusFailed), step("1.2", ""))
	if _, _, ok := c.NextRunnable(); ok {
		t.Fatal("walk must halt at a failed step, not advance to 1.2")
	}
}

func TestNextRunnableComplete(t *testing.T) {
	c := cyc(step("1.1", StatusDone), step("1.2", StatusSkipped))
	if _, _, ok := c.NextRunnable(); ok {
		t.Fatal("all-terminal cycle must report complete")
	}
}

func TestResetStuck(t *testing.T) {
	c := cyc(step("1.1", StatusInProgress), step("1.2", StatusDone))
	if n := c.ResetStuck(); n != 1 {
		t.Fatalf("want 1 reset, got %d", n)
	}
	if c.Phases[0].Steps[0].Status != StatusPending {
		t.Fatal("stuck in_progress must reset to pending")
	}
}

func TestMutatorsMoveAddDeleteValidate(t *testing.T) {
	c := &Cycle{ID: "t", Phases: []Phase{
		{ID: "p1", Name: "A", Steps: []Step{step("a", StatusDone), step("b", "")}},
		{ID: "p2", Name: "B"},
	}}
	id, err := c.AddStep("p2", Step{Name: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if _, s := c.FindStep(id); s == nil {
		t.Fatal("added step not found")
	}
	if err := c.MoveStep("a", "p2", 0); err != nil {
		t.Fatal(err)
	}
	if c.Phases[1].Steps[0].ID != "a" {
		t.Fatalf("move failed: %v", c.Phases[1].Steps)
	}
	if !c.DeleteStep("b") {
		t.Fatal("delete b failed")
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid cycle rejected: %v", err)
	}
	// Duplicate id must be rejected.
	c.Phases[0].Steps = append(c.Phases[0].Steps, Step{ID: "a", Name: "dup"})
	if err := c.Validate(); err == nil {
		t.Fatal("duplicate step id must fail validation")
	}
}

func TestMoveStepLeavesCycleUnchangedWhenTargetPhaseIsMissing(t *testing.T) {
	c := &Cycle{ID: "t", Phases: []Phase{{
		ID: "p1", Name: "A", Steps: []Step{step("a", StatusPending), step("b", StatusPending)},
	}}}

	if err := c.MoveStep("a", "missing", 0); err == nil {
		t.Fatal("MoveStep should reject an unknown target phase")
	}
	if got := c.Phases[0].Steps; len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("failed move changed cycle steps: %+v", got)
	}
}

func TestCyclePersistenceRejectsPathLikeIDs(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, id := range []string{"../escape", "nested/cycle", `nested\\cycle`, "/tmp/cycle", ".."} {
		if err := SaveCycle(&Cycle{ID: id}); err == nil {
			t.Errorf("SaveCycle(%q) accepted a path-like id", id)
		}
		if _, err := LoadCycle(id); err == nil {
			t.Errorf("LoadCycle(%q) accepted a path-like id", id)
		}
	}

	if err := SaveCycle(&Cycle{ID: "cycle-safe_1"}); err != nil {
		t.Fatalf("SaveCycle rejected a valid id: %v", err)
	}
}
