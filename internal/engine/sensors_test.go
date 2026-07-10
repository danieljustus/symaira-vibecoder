package engine

import (
	"context"
	"os/exec"
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

func TestPredicateHolds(t *testing.T) {
	cases := []struct {
		val  int
		when string
		want bool
	}{
		{0, "==0", true}, {1, "==0", false},
		{3, ">0", true}, {0, ">0", false},
		{5, ">=5", true}, {4, ">=5", false},
		{2, "<=2", true}, {3, "<=2", false},
		{0, "!=0", false}, {1, "!=0", true},
		{0, "clean", true}, {2, "clean", false},
		{2, "changed", true}, {0, "changed", false},
		{7, "7", true}, {7, "8", false},
		{1, "garbage", false},
	}
	for _, c := range cases {
		if got := predicateHolds(c.val, c.when); got != c.want {
			t.Errorf("predicateHolds(%d,%q)=%v want %v", c.val, c.when, got, c.want)
		}
	}
}

func TestSensorNamesRegistered(t *testing.T) {
	if len(SensorNames()) < 3 {
		t.Fatalf("expected the core sensors to be registered, got %v", SensorNames())
	}
}

func TestEvalAutoSkipNilRule(t *testing.T) {
	skip, reason, err := EvalAutoSkip(context.Background(), nil, "")
	if err != nil || skip || reason != "" {
		t.Fatalf("nil rule: skip=%v reason=%q err=%v", skip, reason, err)
	}
}

func TestEvalAutoSkipEmptySensor(t *testing.T) {
	rule := &config.AutoSkip{Sensor: "", When: "==0"}
	skip, reason, err := EvalAutoSkip(context.Background(), rule, "")
	if err != nil || skip || reason != "" {
		t.Fatalf("empty sensor: skip=%v reason=%q err=%v", skip, reason, err)
	}
}

func TestEvalAutoSkipUnknownSensor(t *testing.T) {
	rule := &config.AutoSkip{Sensor: "unknown-sensor", When: "==0"}
	_, _, err := EvalAutoSkip(context.Background(), rule, "")
	if err == nil {
		t.Fatal("expected error for unknown sensor")
	}
}

func TestEvalAutoSkipGitDirtySensor(t *testing.T) {
	ctx := context.Background()
	rule := &config.AutoSkip{Sensor: "git-dirty", When: "changed"}
	_, _, err := EvalAutoSkip(ctx, rule, "")
	if err != nil {
		t.Fatalf("git-dirty sensor failed: %v", err)
	}
}

func TestEvalAutoSkipPredicateHolds(t *testing.T) {
	rule := &config.AutoSkip{Sensor: "git-dirty", When: ">=-1"}
	skip, reason, err := EvalAutoSkip(context.Background(), rule, "")
	if err != nil {
		t.Fatalf("EvalAutoSkip error: %v", err)
	}
	if !skip || reason == "" {
		t.Fatalf("expected skip with reason, got skip=%v reason=%q", skip, reason)
	}
}

func TestCountNonEmptyLines(t *testing.T) {
	if got := countNonEmptyLines([]byte("a\n\n b \n\nc")); got != 3 {
		t.Fatalf("countNonEmptyLines: got %d, want 3", got)
	}
	if got := countNonEmptyLines([]byte("   \n\n")); got != 0 {
		t.Fatalf("countNonEmptyLines: got %d, want 0", got)
	}
}

func TestAtoiTrim(t *testing.T) {
	if got := atoiTrim([]byte("  42\n")); got != 42 {
		t.Fatalf("atoiTrim: got %d, want 42", got)
	}
	if got := atoiTrim([]byte("not-a-number")); got != 0 {
		t.Fatalf("atoiTrim: got %d, want 0", got)
	}
}

func TestRunInNotFound(t *testing.T) {
	_, err := runIn(context.Background(), "", "symvibe-definitely-not-found")
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestRunInDir(t *testing.T) {
	out, err := runIn(context.Background(), ".", "git", "rev-parse", "--git-dir")
	if err != nil {
		t.Fatalf("runIn failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("runIn produced no output")
	}
}

func TestLookPathHelper(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git not found on PATH; tests require git")
	}
}
