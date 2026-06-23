package engine

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

// A sensor is a cheap, side-effect-free probe returning an integer metric used
// by a step's AutoSkip rule. Sensors are how the cycle "knows what to skip"
// without hard-coding it.
type sensorFn func(ctx context.Context, dir string) (int, error)

var sensors = map[string]sensorFn{
	"git-dirty":   sensorGitDirty,   // count of changed files (porcelain)
	"git-ahead":   sensorGitAhead,   // commits ahead of upstream
	"open-issues": sensorOpenIssues, // open GitHub issues (needs gh)
	"open-prs":    sensorOpenPRs,    // open GitHub PRs (needs gh)
}

// SensorNames lists the registered sensors (for docs / the GUI rule editor).
func SensorNames() []string {
	out := make([]string, 0, len(sensors))
	for k := range sensors {
		out = append(out, k)
	}
	return out
}

// EvalAutoSkip evaluates a step's rule. It returns skip=true with a reason when
// the sensor value satisfies the predicate. A sensor error returns err!=nil and
// skip=false: the engine then RUNS the step (fail-open) rather than silently
// skipping work it could not justify skipping.
func EvalAutoSkip(ctx context.Context, rule *config.AutoSkip, dir string) (skip bool, reason string, err error) {
	if rule == nil || rule.Sensor == "" {
		return false, "", nil
	}
	fn, ok := sensors[rule.Sensor]
	if !ok {
		return false, "", fmt.Errorf("unknown sensor %q", rule.Sensor)
	}
	val, err := fn(ctx, dir)
	if err != nil {
		return false, "", err
	}
	if predicateHolds(val, rule.When) {
		return true, fmt.Sprintf("%s=%d satisfies %q", rule.Sensor, val, rule.When), nil
	}
	return false, "", nil
}

// predicateHolds parses comparisons like "==0", ">0", ">=3", "!=0", plus the
// shorthands "changed" (>0), "clean" (==0) and a bare integer (equality).
func predicateHolds(val int, when string) bool {
	when = strings.TrimSpace(when)
	switch when {
	case "changed":
		return val > 0
	case "clean":
		return val == 0
	}
	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<"} {
		if strings.HasPrefix(when, op) {
			n, err := strconv.Atoi(strings.TrimSpace(when[len(op):]))
			if err != nil {
				return false
			}
			switch op {
			case "==":
				return val == n
			case "!=":
				return val != n
			case ">=":
				return val >= n
			case "<=":
				return val <= n
			case ">":
				return val > n
			case "<":
				return val < n
			}
		}
	}
	if n, err := strconv.Atoi(when); err == nil {
		return val == n
	}
	return false
}

func sensorGitDirty(ctx context.Context, dir string) (int, error) {
	out, err := runIn(ctx, dir, "git", "status", "--porcelain")
	if err != nil {
		return 0, err
	}
	return countNonEmptyLines(out), nil
}

func sensorGitAhead(ctx context.Context, dir string) (int, error) {
	out, err := runIn(ctx, dir, "git", "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return 0, err
	}
	return atoiTrim(out), nil
}

func sensorOpenIssues(ctx context.Context, dir string) (int, error) {
	out, err := runIn(ctx, dir, "gh", "issue", "list", "--state", "open", "--limit", "200", "--json", "number", "-q", "length")
	if err != nil {
		return 0, err
	}
	return atoiTrim(out), nil
}

func sensorOpenPRs(ctx context.Context, dir string) (int, error) {
	out, err := runIn(ctx, dir, "gh", "pr", "list", "--state", "open", "--limit", "200", "--json", "number", "-q", "length")
	if err != nil {
		return 0, err
	}
	return atoiTrim(out), nil
}

func runIn(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func countNonEmptyLines(b []byte) int {
	n := 0
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
}

func atoiTrim(b []byte) int {
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}
