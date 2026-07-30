// Package config — workspace (cycle working-tree) configuration.
package config

import "fmt"

// WorkspaceMode controls where a cycle runs its steps.
type WorkspaceMode string

const (
	// WorkspaceShared runs steps in the engine's configured working directory
	// (the current behaviour). This is the default.
	WorkspaceShared WorkspaceMode = "shared"

	// WorkspaceIsolated runs steps in a dedicated linked git worktree created
	// and managed per cycle run.
	WorkspaceIsolated WorkspaceMode = "isolated"
)

// IsValid reports whether the mode is one of the known values.
func (m WorkspaceMode) IsValid() bool {
	switch m {
	case WorkspaceShared, WorkspaceIsolated:
		return true
	}
	return false
}

// WorkspaceConfig holds the per-cycle workspace strategy. A zero value means
// "shared" (the historical default).
type WorkspaceConfig struct {
	// Mode selects shared or isolated execution. Empty defaults to shared.
	Mode WorkspaceMode `toml:"mode,omitempty" json:"mode,omitempty"`

	// WorktreeDir is the project-local directory for linked worktrees when
	// Mode is "isolated". If empty, the engine defaults to ".worktrees/"
	// inside the project root.
	WorktreeDir string `toml:"worktree_dir,omitempty" json:"worktree_dir,omitempty"`
}

// Validate checks workspace configuration consistency.
func (w WorkspaceConfig) Validate() error {
	if w.Mode == "" || w.Mode == WorkspaceShared {
		return nil // shared is always valid
	}
	if !w.Mode.IsValid() {
		return fmt.Errorf("workspace: unknown mode %q (want shared|isolated)", w.Mode)
	}
	return nil
}

// EffectiveMode returns the effective workspace mode for the cycle, defaulting
// to shared when unset.
func (w WorkspaceConfig) EffectiveMode() WorkspaceMode {
	if w.Mode == "" {
		return WorkspaceShared
	}
	return w.Mode
}
