// Package runner abstracts the coding-agent backend that actually executes a
// cycle step. Supported backends: OpenCodeRunner (opencode), APIRunner (Anthropic
// API), CLIRunner (aider, claudecode, cline), and LocalAPIRunner (Ollama, LM
// Studio, MLX). The Runner interface is the single swap point for all backends.
// symvibe owns the orchestration; the backend is a peer it drives, never a
// compile-time dependency on agent internals.
package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

// ErrUnavailable is returned by RunStep when the backend cannot be located.
var ErrUnavailable = errors.New("runner: backend unavailable")

// RunEventKind is the normalized, transport-agnostic class of a streamed event.
type RunEventKind string

const (
	EventStart RunEventKind = "start" // the run process started
	EventLog   RunEventKind = "log"   // a human-readable progress line
	EventTool  RunEventKind = "tool"  // a tool/command was invoked
	EventAgent RunEventKind = "agent" // a subagent was spawned / is working
	EventError RunEventKind = "error" // an error surfaced from the backend
	EventDone  RunEventKind = "done"  // TERMINAL: the run finished (see Err)
)

// RunEvent is one normalized event from a running step. The final event on the
// channel is always EventDone; its Err is "" on success and non-empty on
// failure (timeout, backend error event, or non-zero exit). Intermediate
// EventError events are informational — the terminal EventDone.Err is
// authoritative for the step's outcome.
type RunEvent struct {
	Kind RunEventKind `json:"kind"`
	Text string       `json:"text,omitempty"` // human-readable summary
	Err  string       `json:"err,omitempty"`  // cause, on EventError / failed EventDone
	Raw  string       `json:"raw,omitempty"`  // original backend payload (debug/feed)
}

// StepRequest is the backend-agnostic descriptor for one step attempt. The
// engine builds it from a config.RunSpec (model resolution already applied) and
// a fully-composed Message (skill trigger + prompt suffix).
type StepRequest struct {
	RunID      string // symvibe-side correlation id
	StepID     string // cycle step id (e.g. "1.1")
	Skill      string // opencode skill name, for logging (e.g. "00-sync"); "" => free prompt
	Agent      string // opencode agent name (e.g. "build"); may be ""
	Model      string // "provider/model"; "" => backend default
	Variant    string // "" | high | max | minimal
	Message    string // the composed prompt sent to the backend
	WorkingDir string // repo the step operates on
	SkipPerms  bool   // pass --dangerously-skip-permissions (unattended runs)
}

// Info describes a located backend for the doctor / availability surface.
type Info struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Path    string `json:"path,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// Runner drives a coding-agent backend.
type Runner interface {
	// Name is the backend identifier ("opencode").
	Name() string

	// Available reports whether the backend can run, with version/path info for
	// the doctor surface. Never errors — absence is normal (graceful degradation).
	Available(ctx context.Context) (bool, Info)

	// RunStep executes one step and returns a channel of normalized events that
	// is closed when the run terminates. The caller cancels by cancelling ctx.
	// Returns ErrUnavailable (and a nil channel) when the backend is missing.
	RunStep(ctx context.Context, req StepRequest) (<-chan RunEvent, error)
}

// ErrUnsupportedBackend is returned by New when the configured backend is not
// one of the supported values ("opencode", "api").
var ErrUnsupportedBackend = errors.New("runner: unsupported backend")

// New creates the configured backend. It is the single factory for Runner
// implementations; serve.go and doctor.go use it instead of hardcoding one.
// Returns ErrUnsupportedBackend for unknown backend values.
func New(cfg config.RunnerConfig) (Runner, error) {
	timeout := cfg.RequestTimeout.Std()
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	switch cfg.Backend {
	case "opencode", "":
		return NewOpenCodeRunner(cfg.OpencodeBin, timeout), nil
	case "api":
		return NewAPIRunner(cfg.APIKey, timeout), nil
	case "claudecode":
		bin := resolveCLIBin(cfg.ClaudeCodeBin, "claude")
		if bin == "" {
			return nil, fmt.Errorf("%w: %q (claude binary not found)", ErrUnsupportedBackend, cfg.Backend)
		}
		return NewCLIRunner(CLIConfig{
			Name:    "claudecode",
			BinPath: bin,
			Timeout: timeout,
			BuildArgs: func(req StepRequest) []string {
				args := []string{"-p", req.Message, "--dangerously-skip-permissions", "--bare"}
				if req.WorkingDir != "" {
					args = append(args, "--dir", req.WorkingDir)
				}
				return args
			},
		}), nil
	case "aider":
		bin := resolveCLIBin(cfg.AiderBin, "aider")
		if bin == "" {
			return nil, fmt.Errorf("%w: %q (aider binary not found)", ErrUnsupportedBackend, cfg.Backend)
		}
		return NewCLIRunner(CLIConfig{
			Name:    "aider",
			BinPath: bin,
			Timeout: timeout,
			BuildArgs: func(req StepRequest) []string {
				args := []string{"--message", req.Message, "--yes-always"}
				if req.Model != "" {
					args = append(args, "--model", req.Model)
				}
				return args
			},
		}), nil
	case "cline":
		bin := resolveCLIBin(cfg.ClineBin, "cline")
		if bin == "" {
			return nil, fmt.Errorf("%w: %q (cline binary not found)", ErrUnsupportedBackend, cfg.Backend)
		}
		return NewCLIRunner(CLIConfig{
			Name:    "cline",
			BinPath: bin,
			Timeout: timeout,
			BuildArgs: func(req StepRequest) []string {
				return []string{req.Message, "--non-interactive"}
			},
		}), nil
	case "local_api":
		return NewLocalAPIRunner(cfg.LocalAPIEndpoint, cfg.LocalAPIToken, cfg.LocalAPIModel, timeout), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedBackend, cfg.Backend)
	}
}
