package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

func TestLedgerRunStartEnd(t *testing.T) {
	tmpDir := t.TempDir()
	ledger := NewLedger(tmpDir)

	ledger.RunStarted("run_test_1", "cycle-1", "cycle", "shared")

	// Verify the JSONL file was created.
	entries := readLedgerLines(t, ledger, "run_test_1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after run start, got %d", len(entries))
	}
	if entries[0].Type != EntryRunStart {
		t.Fatalf("entry type = %q, want run_start", entries[0].Type)
	}
	if entries[0].RunID != "run_test_1" {
		t.Fatalf("run_id = %q, want run_test_1", entries[0].RunID)
	}
	if entries[0].CycleID != "cycle-1" {
		t.Fatalf("cycle_id = %q, want cycle-1", entries[0].CycleID)
	}

	// Run end.
	ledger.RunEnded(false)

	// Check that run_end entry exists.
	entries = readLedgerLines(t, ledger, "run_test_1")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after run end, got %d", len(entries))
	}
	if entries[1].Type != EntryRunEnd {
		t.Fatalf("entry type = %q, want run_end", entries[1].Type)
	}
	if entries[1].Status != "complete" {
		t.Fatalf("status = %q, want complete", entries[1].Status)
	}

	// Check that summary file was written.
	summary, err := ledger.LatestSummary()
	if err != nil {
		t.Fatalf("LatestSummary: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.RunID != "run_test_1" {
		t.Fatalf("summary run_id = %q, want run_test_1", summary.RunID)
	}
	if summary.Status != "complete" {
		t.Fatalf("summary status = %q, want complete", summary.Status)
	}
	if summary.Mode != "cycle" {
		t.Fatalf("summary mode = %q, want cycle", summary.Mode)
	}
}

func TestLedgerStepLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	ledger := NewLedger(tmpDir)
	ledger.RunStarted("run_test_2", "cycle-1", "step", "shared")

	ledger.StepStarted("s1", "Step One", "deep", "opencode", "opencode-go/deepseek-v4-flash", "high", 1)
	ledger.StepDone("s1", config.StatusDone, "", "", "", &UsageRecord{
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      0.002,
		Model:        "opencode-go/deepseek-v4-flash",
	})

	// Wait a tick for timestamps.
	time.Sleep(time.Millisecond)

	ledger.StepStarted("s2", "Step Two", "git", "opencode", "opencode-go/mimo-v2.5", "", 1)
	ledger.StepDone("s2", config.StatusSkipped, "", "", "git-clean", nil)

	ledger.RunEnded(false)

	// Check the summary.
	summary, err := ledger.LatestSummary()
	if err != nil {
		t.Fatalf("LatestSummary: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if len(summary.Steps) != 2 {
		t.Fatalf("expected 2 steps in summary, got %d", len(summary.Steps))
	}

	// Verify first step summary.
	s1 := summary.Steps[0]
	if s1.StepID != "s1" {
		t.Fatalf("step[0].step_id = %q, want s1", s1.StepID)
	}
	if s1.FinalStatus != "done" {
		t.Fatalf("step[0].status = %q, want done", s1.FinalStatus)
	}
	if s1.CostUSD != 0.002 {
		t.Fatalf("step[0].cost = %f, want 0.002", s1.CostUSD)
	}

	// Verify second step summary.
	s2 := summary.Steps[1]
	if s2.StepID != "s2" {
		t.Fatalf("step[1].step_id = %q, want s2", s2.StepID)
	}
	if s2.FinalStatus != "skipped" {
		t.Fatalf("step[1].status = %q, want skipped", s2.FinalStatus)
	}
	if s2.SensorSkip != "git-clean" {
		t.Fatalf("step[1].sensor_skip = %q, want git-clean", s2.SensorSkip)
	}
}

func TestLedgerCancelledRun(t *testing.T) {
	tmpDir := t.TempDir()
	ledger := NewLedger(tmpDir)

	ledger.RunStarted("run_cancel", "cycle-1", "cycle", "shared")
	ledger.RunEnded(true)

	summary, err := ledger.LatestSummary()
	if err != nil {
		t.Fatalf("LatestSummary: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.Status != "cancelled" {
		t.Fatalf("summary status = %q, want cancelled", summary.Status)
	}
	if !summary.Cancelled {
		t.Fatal("expected Cancelled=true")
	}
}

func TestLedgerHaltedRun(t *testing.T) {
	tmpDir := t.TempDir()
	ledger := NewLedger(tmpDir)

	ledger.RunStarted("run_halt", "cycle-1", "cycle", "shared")
	ledger.StepStarted("s1", "Step One", "deep", "opencode", "mimo", "", 1)
	ledger.StepDone("s1", config.StatusFailed, "exhausted", "all models failed", "", nil)
	ledger.RunEnded(false)

	summary, err := ledger.LatestSummary()
	if err != nil {
		t.Fatalf("LatestSummary: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if summary.Status != "halted" {
		t.Fatalf("summary status = %q, want halted", summary.Status)
	}
}

func TestLedgerNoSummaryDir(t *testing.T) {
	tmpDir := t.TempDir()
	ledger := NewLedger(filepath.Join(tmpDir, "nonexistent"))

	summary, err := ledger.LatestSummary()
	if err != nil {
		t.Fatalf("LatestSummary: %v", err)
	}
	if summary != nil {
		t.Fatal("expected nil summary when ledger dir does not exist")
	}
}

func TestLedgerTruncateErr(t *testing.T) {
	short := "short error"
	got := truncateErr(short)
	if got != "short error" {
		t.Fatalf("truncateErr(%q) = %q, want %q", short, got, short)
	}

	long := strings.Repeat("x", 250)
	got = truncateErr(long)
	if len(got) != 203 { // 200 + "..."
		t.Fatalf("truncated length = %d, want 203", len(got))
	}
	if got[200:] != "..." {
		t.Fatalf("truncated suffix = %q, want ...", got[200:])
	}
}

func TestLedgerFmtDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m0s"},
		{1*time.Hour + 30*time.Minute + 15*time.Second, "1h30m15s"},
		{0, "0s"},
	}
	for _, tc := range tests {
		got := fmtDuration(tc.d)
		if got != tc.want {
			t.Errorf("fmtDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func readLedgerLines(t *testing.T, ledger *Ledger, runID string) []LedgerEntry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ledger.ledgerDir, runID+".jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var entries []LedgerEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e LedgerEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}
