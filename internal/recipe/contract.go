// Package recipe defines the reviewed-recipe runner contract for vault
// automation. An MCP caller (e.g. SymDesk) sends a RecipeRequest describing a
// prompt to run, a workspace, tool constraints, and a review mode flag. The
// service executes the recipe against the configured runner, collects a
// replayable trace, captures any workspace changes as a proposed diff, and — in
// review mode — restores the workspace to its pre-run state.
package recipe

import (
	"path/filepath"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

// SchemaVersion is the current contract version. Callers must send this in
// every RecipeRequest so the service can validate or migrate.
const SchemaVersion = "1"

// WriteCap describes the maximum write scope the recipe is allowed to perform.
type WriteCap string

const (
	WriteCapNone      WriteCap = "none"      // read-only: no file writes allowed
	WriteCapWorkspace WriteCap = "workspace" // may write within the workspace
	WriteCapFull      WriteCap = "full"      // unrestricted writes
)

// RecipeRequest is the versioned inbound contract from an MCP caller. Every
// field is optional except Prompt and Workspace; zero values use sensible
// defaults.
type RecipeRequest struct {
	SchemaVersion string   `json:"schema_version"`            // must be "1"
	Workspace     string   `json:"workspace"`                 // repo / working directory
	Prompt        string   `json:"prompt"`                    // instruction sent to the backend
	ToolAllowList []string `json:"tool_allow_list,omitempty"` // opaque tool names; echoed in result
	WriteCap      WriteCap `json:"write_cap,omitempty"`       // "none" | "workspace" | "full"; default workspace
	TracePath     string   `json:"trace_path,omitempty"`      // when set, write replayable trace JSON here
	ReviewMode    bool     `json:"review_mode,omitempty"`     // when true, restore workspace after capturing diff
	Model         string   `json:"model,omitempty"`           // optional model override ("provider/model")
	Agent         string   `json:"agent,omitempty"`           // optional agent override
	Variant       string   `json:"variant,omitempty"`         // optional variant ("high", "max", "minimal")
	SkipPerms     bool     `json:"skip_perms,omitempty"`      // pass --dangerously-skip-permissions
}

// RecipeResult is the structured response returned to the caller after the
// recipe run completes (or fails). The caller can inspect the trace, proposed
// diff, and effective constraints.
type RecipeResult struct {
	RunID         string            `json:"run_id"`
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"` // "done" | "failed"
	Trace         []runner.RunEvent `json:"trace"`
	TracePath     string            `json:"trace_path,omitempty"`
	ProposedDiff  string            `json:"proposed_diff,omitempty"`
	ToolAllowList []string          `json:"tool_allow_list,omitempty"`
	WriteCap      WriteCap          `json:"write_cap,omitempty"`
	Error         string            `json:"error,omitempty"`
	Backend       string            `json:"backend"`
	Duration      time.Duration     `json:"duration"`
}

// Validate checks required fields and returns an error describing the first
// violation. It normalizes zero-value fields to defaults.
func (r *RecipeRequest) Validate() error {
	if r.SchemaVersion == "" {
		r.SchemaVersion = SchemaVersion
	}
	if r.SchemaVersion != SchemaVersion {
		return ErrInvalidSchema
	}
	if r.Prompt == "" {
		return ErrMissingPrompt
	}
	if r.Workspace == "" {
		return ErrMissingWorkspace
	}
	if !filepath.IsAbs(r.Workspace) {
		return ErrInvalidWorkspace
	}
	if r.WriteCap == "" {
		r.WriteCap = WriteCapWorkspace
	}
	if r.TracePath != "" && !filepath.IsLocal(r.TracePath) {
		return ErrInvalidTracePath
	}
	return nil
}
