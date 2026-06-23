package config

import (
	"fmt"
	"strconv"
)

// Board edit operations on a Cycle. These back the GUI's add / remove / move /
// reorder actions. After any mutation the caller persists with SaveCycle.

// Validate checks structural integrity: non-empty cycle id and globally-unique
// phase and step ids. Call before SaveCycle on a GUI-supplied cycle.
func (c *Cycle) Validate() error {
	if err := validateCycleID(c.ID); err != nil {
		return err
	}
	seenP := map[string]bool{}
	seenS := map[string]bool{}
	for pi := range c.Phases {
		p := &c.Phases[pi]
		if p.ID == "" {
			return fmt.Errorf("cycle: phase #%d has empty id", pi+1)
		}
		if seenP[p.ID] {
			return fmt.Errorf("cycle: duplicate phase id %q", p.ID)
		}
		seenP[p.ID] = true
		for si := range p.Steps {
			s := &p.Steps[si]
			if s.ID == "" {
				return fmt.Errorf("cycle: a step in phase %q has an empty id", p.ID)
			}
			if seenS[s.ID] {
				return fmt.Errorf("cycle: duplicate step id %q", s.ID)
			}
			seenS[s.ID] = true
		}
	}
	return nil
}

// Reindex renumbers phase and step order fields (1-based) to match slice order.
func (c *Cycle) Reindex() {
	for pi := range c.Phases {
		c.Phases[pi].Order = pi + 1
		for si := range c.Phases[pi].Steps {
			c.Phases[pi].Steps[si].Order = si + 1
		}
	}
}

func (c *Cycle) phaseByID(id string) *Phase {
	for i := range c.Phases {
		if c.Phases[i].ID == id {
			return &c.Phases[i]
		}
	}
	return nil
}

// AddStep appends s to the named phase, assigning a unique id when empty.
func (c *Cycle) AddStep(phaseID string, s Step) (string, error) {
	p := c.phaseByID(phaseID)
	if p == nil {
		return "", fmt.Errorf("phase %q not found", phaseID)
	}
	if s.ID == "" {
		s.ID = c.newStepID()
	} else if _, dup := c.FindStep(s.ID); dup != nil {
		return "", fmt.Errorf("duplicate step id %q", s.ID)
	}
	if s.Status == "" {
		s.Status = StatusPending
	}
	p.Steps = append(p.Steps, s)
	c.Reindex()
	return s.ID, nil
}

// DeleteStep removes the step with stepID. Returns false if not found.
func (c *Cycle) DeleteStep(stepID string) bool {
	for pi := range c.Phases {
		for si := range c.Phases[pi].Steps {
			if c.Phases[pi].Steps[si].ID == stepID {
				c.Phases[pi].Steps = append(c.Phases[pi].Steps[:si], c.Phases[pi].Steps[si+1:]...)
				c.Reindex()
				return true
			}
		}
	}
	return false
}

// MoveStep moves a step to toPhaseID at position toIndex (clamped to the target
// phase's bounds). Works within and across phases — backs board drag & drop.
func (c *Cycle) MoveStep(stepID, toPhaseID string, toIndex int) error {
	dst := c.phaseByID(toPhaseID)
	if dst == nil {
		return fmt.Errorf("target phase %q not found", toPhaseID)
	}

	var moved Step
	found := false
	for pi := range c.Phases {
		for si := range c.Phases[pi].Steps {
			if c.Phases[pi].Steps[si].ID == stepID {
				moved = c.Phases[pi].Steps[si]
				c.Phases[pi].Steps = append(c.Phases[pi].Steps[:si], c.Phases[pi].Steps[si+1:]...)
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return fmt.Errorf("step %q not found", stepID)
	}
	dst.Steps = insertStep(dst.Steps, moved, toIndex)
	c.Reindex()
	return nil
}

// DuplicateStep clones a step (new id, "(copy)" name, pending status) right
// after the original. Returns the new id.
func (c *Cycle) DuplicateStep(stepID string) (string, error) {
	for pi := range c.Phases {
		for si := range c.Phases[pi].Steps {
			if c.Phases[pi].Steps[si].ID == stepID {
				clone := c.Phases[pi].Steps[si]
				clone.ID = c.newStepID()
				clone.Name += " (copy)"
				clone.Status = StatusPending
				c.Phases[pi].Steps = insertStep(c.Phases[pi].Steps, clone, si+1)
				c.Reindex()
				return clone.ID, nil
			}
		}
	}
	return "", fmt.Errorf("step %q not found", stepID)
}

// AddPhase appends a new, empty phase with the given name. Returns its id.
func (c *Cycle) AddPhase(name string) string {
	id := c.newPhaseID()
	c.Phases = append(c.Phases, Phase{ID: id, Name: name})
	c.Reindex()
	return id
}

// DeletePhase removes a phase and all its steps. Returns false if not found.
func (c *Cycle) DeletePhase(phaseID string) bool {
	for pi := range c.Phases {
		if c.Phases[pi].ID == phaseID {
			c.Phases = append(c.Phases[:pi], c.Phases[pi+1:]...)
			c.Reindex()
			return true
		}
	}
	return false
}

func insertStep(steps []Step, s Step, at int) []Step {
	if at < 0 {
		at = 0
	}
	if at > len(steps) {
		at = len(steps)
	}
	steps = append(steps, Step{})
	copy(steps[at+1:], steps[at:])
	steps[at] = s
	return steps
}

func (c *Cycle) newStepID() string {
	for n := 1; ; n++ {
		id := "s" + strconv.Itoa(n)
		if _, dup := c.FindStep(id); dup == nil {
			return id
		}
	}
}

func (c *Cycle) newPhaseID() string {
	for n := 1; ; n++ {
		id := "p" + strconv.Itoa(n)
		if c.phaseByID(id) == nil {
			return id
		}
	}
}
