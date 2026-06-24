package runner

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"time"
)

// OpenCodeRunner is the default backend. It drives the local `opencode` binary
// one step at a time via `opencode run --format json`, scanning the
// newline-delimited JSON event stream on stdout into normalized RunEvents.
//
// Notes grounded in opencode 1.17.9:
//   - It does NOT pass --pure: --pure disables the auth/subscription plugin and
//     every zen model returns "Model is disabled" (401). The user's own agents
//     and provider auth must load.
//   - --dangerously-skip-permissions (when SkipPerms) prevents an unattended run
//     from blocking on a tool-permission prompt with no TTY. See SECURITY.md.
//   - --format json events are read line-by-line; the engine owns the
//     pending->in_progress->done/failed transitions around process lifecycle,
//     so the step status is correct whether opencode streams live or buffers.
type OpenCodeRunner struct {
	bin     string        // resolved opencode path ("" => unavailable)
	timeout time.Duration // per-step wall-clock budget; 0 => no timeout
}

// NewOpenCodeRunner resolves the opencode binary from the configured path (or
// PATH / ~/.opencode/bin) and binds a per-step timeout.
func NewOpenCodeRunner(configuredBin string, timeout time.Duration) *OpenCodeRunner {
	return &OpenCodeRunner{bin: ResolveOpencodeBin(configuredBin), timeout: timeout}
}

func (r *OpenCodeRunner) Name() string { return "opencode" }

// Available probes the binary and its version. Never errors.
func (r *OpenCodeRunner) Available(ctx context.Context) (bool, Info) {
	if r.bin == "" {
		return false, Info{Name: "opencode", Detail: "not found on PATH or ~/.opencode/bin"}
	}
	ver, ok := OpencodeVersion(r.bin)
	if !ok {
		return false, Info{Name: "opencode", Path: r.bin, Detail: "could not run --version"}
	}
	versionOK, detail := CheckOpencodeVersion(ver)
	info := Info{Name: "opencode", Version: ver, Path: r.bin, Detail: detail}
	if !versionOK {
		return false, info
	}
	return true, info
}

// buildArgs assembles the `opencode run` invocation. The composed Message is the
// last positional. The bound skill is invoked through the message (the engine
// prefixes "$<skill>"), so --command is intentionally not used in the MVP (a
// skill folder name is not a verified --command value, and a skill's own SKILL.md
// binding may otherwise shadow --model).
func buildArgs(req StepRequest) []string {
	args := []string{"run", "--format", "json"}
	if req.Agent != "" {
		args = append(args, "--agent", req.Agent)
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.Variant != "" {
		args = append(args, "--variant", req.Variant)
	}
	if req.WorkingDir != "" {
		args = append(args, "--dir", req.WorkingDir)
	}
	if req.SkipPerms {
		args = append(args, "--dangerously-skip-permissions")
	}
	if msg := strings.TrimSpace(req.Message); msg != "" {
		args = append(args, msg)
	}
	return args
}

// RunStep executes one step, streaming normalized events until the run ends.
func (r *OpenCodeRunner) RunStep(ctx context.Context, req StepRequest) (<-chan RunEvent, error) {
	if r.bin == "" {
		return nil, ErrUnavailable
	}

	runCtx := ctx
	cancel := context.CancelFunc(func() {})
	if r.timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, r.timeout)
	}

	cmd := exec.CommandContext(runCtx, r.bin, buildArgs(req)...)
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	ch := make(chan RunEvent, 128)

	// Drain stderr concurrently so a full stderr pipe never deadlocks stdout.
	var errBuf bytes.Buffer
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errBuf, stderr)
		close(stderrDone)
	}()

	go func() {
		defer close(ch)
		defer cancel()

		emit(ch, runCtx, RunEvent{Kind: EventStart, Text: "opencode run started"})

		var firstErr string
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 8<<20) // events can be large
		for sc.Scan() {
			ev := mapEvent(sc.Bytes())
			if ev.Kind == EventError && firstErr == "" {
				firstErr = firstNonEmpty(ev.Err, ev.Text)
			}
			emit(ch, runCtx, ev)
		}

		<-stderrDone
		waitErr := cmd.Wait()

		done := RunEvent{Kind: EventDone}
		switch {
		case runCtx.Err() == context.DeadlineExceeded:
			done.Err = "step timed out"
		case ctx.Err() == context.Canceled:
			done.Err = "cancelled"
		case firstErr != "":
			done.Err = firstErr
		case waitErr != nil:
			if e := strings.TrimSpace(errBuf.String()); e != "" {
				done.Err = lastLine(e)
			} else {
				done.Err = waitErr.Error()
			}
		}
		if done.Err == "" {
			done.Text = "completed"
		} else {
			done.Text = "failed"
		}
		emit(ch, runCtx, done)
	}()

	return ch, nil
}

// emit sends an event unless the run context is already done (a cancelled run
// may drop late events; the engine treats channel-close as terminal regardless).
func emit(ch chan<- RunEvent, ctx context.Context, ev RunEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
		// best-effort: still try a non-blocking send so the terminal event lands
		select {
		case ch <- ev:
		default:
		}
	}
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}
