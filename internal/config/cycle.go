package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ---------------------------------------------------------------------------
// Baukasten ("cycle") schema — stored per-cycle under the DATA dir so GUI edits
// survive: ~/.local/share/symvibe/cycles/<id>.toml
// ---------------------------------------------------------------------------

// Cycle is one editable Baukasten: an ordered set of phases, each with steps.
// The struct carries both toml (on-disk) and json (on-the-wire) tags using the
// SAME snake_case names so disk and GUI share one mental model.
type Cycle struct {
	SchemaVersion int     `toml:"schema_version" json:"schema_version"`
	ID            string  `toml:"id" json:"id"`
	Name          string  `toml:"name" json:"name"`
	Description   string  `toml:"description" json:"description"`
	Phases        []Phase `toml:"phases" json:"phases"`
}

// Phase is a category/column on the board.
type Phase struct {
	ID    string `toml:"id" json:"id"`
	Name  string `toml:"name" json:"name"`
	Order int    `toml:"order" json:"order"`
	Steps []Step `toml:"steps" json:"steps"`
}

// Step is one block. Skill binds it to an opencode skill/command; Category binds
// it to a model. ModelOverride, when non-nil, beats the category binding.
// Status is RUNTIME state, persisted so the cycle knows where it left off.
type Step struct {
	ID            string `toml:"id" json:"id"`
	Name          string `toml:"name" json:"name"`
	Order         int    `toml:"order" json:"order"`
	Skill         string `toml:"skill" json:"skill"`                 // opencode command/skill name
	Category      string `toml:"category" json:"category"`           // model binding key
	Agent         string `toml:"agent" json:"agent"`                 // optional opencode agent override
	PromptSuffix  string `toml:"prompt_suffix" json:"prompt_suffix"` // optional extra instruction
	Enabled       bool   `toml:"enabled" json:"enabled"`
	ModelOverride *Model `toml:"model_override" json:"model_override,omitempty"` // optional per-step model

	// AutoSkip is an optional data-driven skip rule. Before running the step the
	// engine evaluates the named Sensor; if its value satisfies When, the step is
	// marked skipped and the runner is never invoked (e.g. sensor="open-issues"
	// when="==0" -> skip when there are no open issues). This is how the cycle
	// "knows what to skip" without hard-coding it.
	AutoSkip *AutoSkip `toml:"auto_skip" json:"auto_skip,omitempty"`

	// DependsOn lists step IDs that must be terminal before this one may run.
	// Reserved for the engine; the MVP walks strictly in board order.
	DependsOn []string `toml:"depends_on" json:"depends_on,omitempty"`

	// ParallelSafe marks a step that may run concurrently with phase siblings.
	// Reserved; the MVP runs one step at a time on a shared working tree.
	ParallelSafe bool `toml:"parallel_safe" json:"parallel_safe,omitempty"`

	// Runtime status (persisted across runs so the cycle is resumable). An empty
	// value means pending — see StepStatus.Effective.
	Status StepStatus `toml:"status" json:"status"`
}

// AutoSkip is a cheap pre-step probe + predicate. The engine runs Sensor (a
// fast, side-effect-free probe such as git-dirty / open-issues / open-prs) and
// skips the step when the integer result satisfies When (e.g. "==0", ">0",
// ">=3", "!=0").
type AutoSkip struct {
	Sensor string `toml:"sensor" json:"sensor"`
	When   string `toml:"when" json:"when"`
}

// StepStatus drives the discreet GUI status icons. The wire/JSON and the
// frontend StepStatus union MUST use these exact string values.
type StepStatus string

const (
	StatusPending     StepStatus = "pending"
	StatusInProgress  StepStatus = "in_progress"
	StatusDone        StepStatus = "done"
	StatusSkipped     StepStatus = "skipped"
	StatusFailed      StepStatus = "failed"
	StatusBlocked     StepStatus = "blocked"
	StatusNeedsReview StepStatus = "needs_review"
)

// Effective normalizes the empty zero-value (a freshly seeded step with no
// status key in its TOML) to pending so it is runnable.
func (s StepStatus) Effective() StepStatus {
	if s == "" {
		return StatusPending
	}
	return s
}

// IsTerminal reports whether the autonomous walk may pass over this step
// because it is finished (done or skipped).
func (s StepStatus) IsTerminal() bool {
	switch s.Effective() {
	case StatusDone, StatusSkipped:
		return true
	}
	return false
}

// IsHalting reports a status that stops the autonomous walk and requires a
// human decision or a resume-reset: failed, blocked, needs_review, or a stale
// in_progress left behind by a crash.
func (s StepStatus) IsHalting() bool {
	switch s.Effective() {
	case StatusFailed, StatusBlocked, StatusNeedsReview, StatusInProgress:
		return true
	}
	return false
}

// LoadCycle reads a cycle by id from the data dir. If it does not exist and id
// is the seed name, it materializes the embedded seed first.
func LoadCycle(id string) (*Cycle, error) {
	path := filepath.Join(CyclesDir(), id+".toml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if id == SeedCycleName {
			if err := materializeSeed(path); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("cycle %q not found at %s", id, path)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cycle %q: %w", id, err)
	}
	var c Cycle
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse cycle %q: %w", id, err)
	}
	return &c, nil
}

// SaveCycle atomically writes a cycle back to the data dir (GUI edit path).
func SaveCycle(c *Cycle) error {
	if c.ID == "" {
		return fmt.Errorf("save cycle: empty id")
	}
	if err := os.MkdirAll(CyclesDir(), 0o755); err != nil {
		return err
	}
	path := filepath.Join(CyclesDir(), c.ID+".toml")
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path) // atomic replace
}

// materializeSeed copies the embedded seed Baukasten to the data dir on first run.
func materializeSeed(dest string) error {
	data, err := seedFS.ReadFile("seed/seed-cycle.toml")
	if err != nil {
		return fmt.Errorf("read embedded seed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

// NextRunnable returns the first enabled, pending step in board order — this is
// how the autonomous cycle "knows where it is". Done/skipped steps are walked
// past; the walk HALTS (ok=false) at the first enabled step in a halting state
// (failed/blocked/needs_review/in_progress) so the cycle never silently runs
// past a problem and never re-selects a failed step forever. Call ResetStuck
// before the first walk of a run to clear any crash-stale in_progress.
// Returns ok=false when the cycle is complete or halted.
func (c *Cycle) NextRunnable() (phaseIdx, stepIdx int, ok bool) {
	for pi := range c.Phases {
		for si := range c.Phases[pi].Steps {
			s := c.Phases[pi].Steps[si]
			if !s.Enabled {
				continue
			}
			switch {
			case s.Status.IsTerminal():
				continue
			case s.Status.IsHalting():
				return 0, 0, false
			default: // pending
				return pi, si, true
			}
		}
	}
	return 0, 0, false
}

// FindStep returns pointers to the phase and step with the given step id, or
// (nil, nil) if not found. The pointers are into the cycle's slices so callers
// may mutate status in place before SaveCycle.
func (c *Cycle) FindStep(stepID string) (*Phase, *Step) {
	for pi := range c.Phases {
		for si := range c.Phases[pi].Steps {
			if c.Phases[pi].Steps[si].ID == stepID {
				return &c.Phases[pi], &c.Phases[pi].Steps[si]
			}
		}
	}
	return nil, nil
}

// ResetStuck flips any persisted in_progress step back to pending so a crash
// mid-run does not wedge the cycle on resume. Returns the number reset.
func (c *Cycle) ResetStuck() (n int) {
	for pi := range c.Phases {
		for si := range c.Phases[pi].Steps {
			if c.Phases[pi].Steps[si].Status == StatusInProgress {
				c.Phases[pi].Steps[si].Status = StatusPending
				n++
			}
		}
	}
	return n
}
