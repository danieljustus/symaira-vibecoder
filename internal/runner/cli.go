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

type CLIConfig struct {
	Name      string   // backend identifier ("aider", "claudecode", "cline")
	BinPath   string   // resolved binary path ("" => unavailable)
	Timeout   time.Duration
	BuildArgs func(req StepRequest) []string // backend-specific arg builder
}

type CLIRunner struct {
	cfg CLIConfig
}

func NewCLIRunner(cfg CLIConfig) *CLIRunner {
	return &CLIRunner{cfg: cfg}
}

func (r *CLIRunner) Name() string { return r.cfg.Name }

func (r *CLIRunner) Available(ctx context.Context) (bool, Info) {
	if r.cfg.BinPath == "" {
		return false, Info{Name: r.cfg.Name, Detail: "not found on PATH"}
	}
	ver := cliVersion(r.cfg.BinPath)
	return true, Info{Name: r.cfg.Name, Version: ver, Path: r.cfg.BinPath}
}

func (r *CLIRunner) RunStep(ctx context.Context, req StepRequest) (<-chan RunEvent, error) {
	if r.cfg.BinPath == "" {
		return nil, ErrUnavailable
	}

	runCtx := ctx
	cancel := context.CancelFunc(func() {})
	if r.cfg.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, r.cfg.Timeout)
	}

	args := r.cfg.BuildArgs(req)
	cmd := exec.CommandContext(runCtx, r.cfg.BinPath, args...)
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
	var errBuf bytes.Buffer
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errBuf, stderr)
		close(stderrDone)
	}()

	go func() {
		defer close(ch)
		defer cancel()

		emit(ch, runCtx, RunEvent{Kind: EventStart, Text: r.cfg.Name + " run started"})

		var firstErr string
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 8<<20)
		for sc.Scan() {
			text := strings.TrimSpace(sc.Text())
			if text == "" {
				continue
			}
			if firstErr == "" && looksLikeError(text) {
				firstErr = text
				emit(ch, runCtx, RunEvent{Kind: EventError, Err: text, Text: text})
			} else {
				emit(ch, runCtx, RunEvent{Kind: EventLog, Text: text})
			}
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

func cliVersion(bin string) string {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		out, err = exec.Command(bin, "-v").Output()
		if err != nil {
			return ""
		}
	}
	return strings.TrimSpace(string(out))
}

func looksLikeError(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "error:") ||
		strings.Contains(lower, "fatal:") ||
		strings.Contains(lower, "panic:")
}

func resolveCLIBin(configured, name string) string {
	if configured != "" {
		if p := expandHome(configured); fileExists(p) {
			return p
		}
		return ""
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}
