package config

import "fmt"

// Resolver turns a Step into the effective ResolvedModel using the precedence:
//
//	step.ModelOverride  >  category binding (step.Category)  >  global default
//
// Category bindings inherit non-set fields from their referenced registry Model.
// The resolver is pure (no I/O) so it is trivially unit-testable.
type Resolver struct {
	cfg *Config
}

// NewResolver binds a resolver to a validated config.
func NewResolver(cfg *Config) *Resolver { return &Resolver{cfg: cfg} }

// Resolve computes the effective model binding for one step.
//
// Precedence:
//  1. step.ModelOverride (a full Model, set on the step) — wins outright.
//  2. The binding for step.Category (or Defaults.Category if step.Category=="").
//  3. As a last resort, the Defaults.Category binding.
//
// A category binding is expanded by starting from its referenced registry Model
// and overlaying any inline category overrides (temperature/variant/fallbacks).
func (r *Resolver) Resolve(step Step) (ResolvedModel, error) {
	// 1) Per-step override beats everything.
	if step.ModelOverride != nil {
		m := *step.ModelOverride
		return ResolvedModel{
			Model:          m.ID,
			Temperature:    m.Temperature,
			Variant:        m.Variant,
			FallbackModels: m.FallbackModels,
			Source:         "override",
		}, nil
	}

	// 2) Category binding (step's category, else the global default category).
	cat := step.Category
	if cat == "" {
		cat = r.cfg.Defaults.Category
	}
	if rm, ok := r.resolveCategory(cat); ok {
		return rm, nil
	}

	// 3) Fall back to the default category explicitly.
	if cat != r.cfg.Defaults.Category {
		if rm, ok := r.resolveCategory(r.cfg.Defaults.Category); ok {
			return rm, nil
		}
	}
	return ResolvedModel{}, fmt.Errorf("resolve: no binding for category %q and default category %q is unresolvable",
		step.Category, r.cfg.Defaults.Category)
}

// resolveCategory expands one category into a ResolvedModel by inheriting from
// its registry model and overlaying inline overrides.
func (r *Resolver) resolveCategory(name string) (ResolvedModel, bool) {
	cat, ok := r.cfg.Categories[name]
	if !ok {
		return ResolvedModel{}, false
	}
	base, ok := r.cfg.Models[cat.ModelRef]
	if !ok {
		return ResolvedModel{}, false
	}
	rm := ResolvedModel{
		Model:          base.ID,
		Temperature:    base.Temperature,
		Variant:        base.Variant,
		FallbackModels: base.FallbackModels,
		Source:         "category:" + name,
	}
	if cat.Temperature != nil {
		rm.Temperature = *cat.Temperature
	}
	if cat.Variant != nil {
		rm.Variant = *cat.Variant
	}
	if cat.FallbackModels != nil {
		rm.FallbackModels = *cat.FallbackModels
	}
	if name == r.cfg.Defaults.Category {
		rm.Source = "default:" + name
	}
	return rm, true
}

// RunSpec is the runner-agnostic call descriptor produced for a single step
// attempt. The OpenCodeRunner maps this onto `opencode run` flags:
//
//	--command <Skill>  --agent <Agent>  --model <Model>  --variant <Variant>
//	--dir <WorkingDir>  (+ temperature via session/model settings)
//
// On failure the caller advances ModelIndex and re-derives the next RunSpec via
// NextAttempt, walking ResolvedModel.Chain() until exhausted.
type RunSpec struct {
	Skill        string // opencode skill/command name (folder under skills/)
	Agent        string // opencode agent (may be "")
	Model        string // current attempt's "provider/model"
	Variant      string // "" | high | max | minimal
	Temperature  float64
	PromptSuffix string // appended to the skill invocation, if any
	WorkingDir   string
	attemptIdx   int // 0 = primary, 1.. = fallback index
}

// BuildRunSpec materializes the primary (first) attempt for a step.
func (r *Resolver) BuildRunSpec(step Step, workingDir string) (RunSpec, ResolvedModel, error) {
	rm, err := r.Resolve(step)
	if err != nil {
		return RunSpec{}, ResolvedModel{}, err
	}
	agent := step.Agent
	if agent == "" {
		agent = r.cfg.Defaults.Agent
	}
	if workingDir == "" {
		workingDir = r.cfg.Runner.WorkingDir
	}
	return RunSpec{
		Skill:        step.Skill,
		Agent:        agent,
		Model:        rm.Model,
		Variant:      rm.Variant,
		Temperature:  rm.Temperature,
		PromptSuffix: step.PromptSuffix,
		WorkingDir:   workingDir,
		attemptIdx:   0,
	}, rm, nil
}

// NextAttempt advances the RunSpec to the next model in the fallback chain.
// Returns ok=false when the chain is exhausted (step should be marked failed).
// Variant/temperature are carried over from the resolved binding; only the
// model id changes as we walk fallbacks.
func NextAttempt(spec RunSpec, rm ResolvedModel) (RunSpec, bool) {
	chain := rm.Chain()
	next := spec.attemptIdx + 1
	if next >= len(chain) {
		return spec, false
	}
	spec.attemptIdx = next
	spec.Model = chain[next]
	return spec, true
}
