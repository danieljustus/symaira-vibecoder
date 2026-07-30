// Package engine — versioned execution ledger for run/step evidence.
//
// The ledger stores two shapes per run:
//  1. Run log — append-only JSONL of every significant event
//  2. Summary — structured record with terminal outcomes per step
//
// Ledger files live under ~/.local/share/symvibe/ledger/.
package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

// LedgerSchemaVersion is the current schema version for ledger entries.
const LedgerSchemaVersion = 1

// EntryType discriminates the kind of ledger entry.
type EntryType string

const (
	EntryRunStart  EntryType = "run_start"
	EntryRunEnd    EntryType = "run_end"
	EntryStepStart EntryType = "step_start"
	EntryStepEnd   EntryType = "step_end"
)

// LedgerEntry is one append-only line in the run log JSONL file.
type LedgerEntry struct {
	SchemaVersion int       `json:"schema_version"`
	Type          EntryType `json:"type"`
	TS            int64     `json:"ts"` // unix millis
	RunID         string    `json:"run_id"`
	CycleID       string    `json:"cycle_id,omitempty"`
	Mode          string    `json:"mode,omitempty"` // step | cycle
	WorkspaceMode string    `json:"workspace_mode,omitempty"`

	// Step fields (set for step_start, step_end)
	StepID   string `json:"step_id,omitempty"`
	StepName string `json:"step_name,omitempty"`
	Category string `json:"category,omitempty"`
	Backend  string `json:"backend,omitempty"`
	Model    string `json:"model,omitempty"`
	Variant  string `json:"variant,omitempty"`
	Attempt  int    `json:"attempt,omitempty"`

	// Terminal fields (set for step_end, run_end)
	Status      string `json:"status,omitempty"`
	ErrorClass  string `json:"error_class,omitempty"`
	ErrorDetail string `json:"error_detail,omitempty"`
	SensorSkip  string `json:"sensor_skip,omitempty"`
	Skipped     bool   `json:"skipped,omitempty"`

	// Token usage (set on step_end when available)
	InputTokens     int     `json:"input_tokens,omitempty"`
	OutputTokens    int     `json:"output_tokens,omitempty"`
	CacheReadTokens int     `json:"cache_read_tokens,omitempty"`
	CostUSD         float64 `json:"cost_usd,omitempty"`
	UsageModel      string  `json:"usage_model,omitempty"`
}

// StepSummary is a condensed record of one step's terminal outcome.
type StepSummary struct {
	StepID      string  `json:"step_id"`
	StepName    string  `json:"step_name"`
	Category    string  `json:"category"`
	Attempts    int     `json:"attempts"`
	FinalStatus string  `json:"final_status"` // done | skipped | failed | needs_review
	Backend     string  `json:"backend"`
	Model       string  `json:"model"`
	Variant     string  `json:"variant"`
	Duration    string  `json:"duration"` // human-readable
	ErrorClass  string  `json:"error_class,omitempty"`
	SensorSkip  string  `json:"sensor_skip,omitempty"`
	CostUSD     float64 `json:"cost_usd"`
}

// RunSummary is the terminal structured record for one run.
type RunSummary struct {
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	CycleID       string        `json:"cycle_id"`
	Mode          string        `json:"mode"`
	WorkspaceMode string        `json:"workspace_mode"`
	StartedAt     int64         `json:"started_at"` // unix millis
	EndedAt       int64         `json:"ended_at"`   // unix millis
	Status        string        `json:"status"`     // complete | halted | failed | cancelled
	Duration      string        `json:"duration"`
	Cancelled     bool          `json:"cancelled"`
	Steps         []StepSummary `json:"steps"`
	TotalCostUSD  float64       `json:"total_cost_usd"`
}

// Ledger manages the versioned execution log for runs and steps.
type Ledger struct {
	mu        sync.Mutex
	ledgerDir string

	// Active run state (in-memory, aggregated from appended entries)
	runID      string
	cycleID    string
	mode       string
	wsMode     string
	startedAt  int64
	steps      []StepSummary
	curStep    string
	curAttempt int
}

// NewLedger creates a ledger rooted at the given directory. Directory is
// created on first write if it does not exist.
func NewLedger(dataDir string) *Ledger {
	return &Ledger{
		ledgerDir: filepath.Join(dataDir, "ledger"),
	}
}

// RunStarted opens a new run in the ledger. Must be called once per run.
func (l *Ledger) RunStarted(runID, cycleID, mode, wsMode string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.runID = runID
	l.cycleID = cycleID
	l.mode = mode
	l.wsMode = wsMode
	l.startedAt = nowMs()
	l.steps = nil
	l.curStep = ""
	l.curAttempt = 0

	_ = l.appendEntryLocked(LedgerEntry{
		SchemaVersion: LedgerSchemaVersion,
		Type:          EntryRunStart,
		TS:            l.startedAt,
		RunID:         runID,
		CycleID:       cycleID,
		Mode:          mode,
		WorkspaceMode: wsMode,
	})
}

// StepStarted records the start of a step attempt.
func (l *Ledger) StepStarted(stepID, stepName, category, backend, model, variant string, attempt int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.curStep = stepID
	l.curAttempt = attempt

	_ = l.appendEntryLocked(LedgerEntry{
		SchemaVersion: LedgerSchemaVersion,
		Type:          EntryStepStart,
		TS:            nowMs(),
		RunID:         l.runID,
		StepID:        stepID,
		StepName:      stepName,
		Category:      category,
		Backend:       backend,
		Model:         model,
		Variant:       variant,
		Attempt:       attempt,
	})
}

// StepDone records the terminal outcome of a step.
func (l *Ledger) StepDone(stepID string, status config.StepStatus, errClass, errDetail, sensorSkip string, usage *UsageRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ts := nowMs()
	entry := LedgerEntry{
		SchemaVersion: LedgerSchemaVersion,
		Type:          EntryStepEnd,
		TS:            ts,
		RunID:         l.runID,
		StepID:        stepID,
		Status:        string(status),
		ErrorClass:    errClass,
		ErrorDetail:   truncateErr(errDetail),
		SensorSkip:    sensorSkip,
		Skipped:       status == config.StatusSkipped,
	}
	if usage != nil {
		entry.InputTokens = usage.InputTokens
		entry.OutputTokens = usage.OutputTokens
		entry.CacheReadTokens = usage.CacheReadTokens
		entry.CostUSD = usage.CostUSD
		entry.UsageModel = usage.Model
	}
	_ = l.appendEntryLocked(entry)

	// Collect into summary.
	duration := ts - l.startedAt
	l.steps = append(l.steps, StepSummary{
		StepID:      stepID,
		FinalStatus: string(status),
		Duration:    fmtDuration(time.Duration(duration) * time.Millisecond),
		ErrorClass:  errClass,
		SensorSkip:  sensorSkip,
		CostUSD:     costIf(usage),
	})
}

// RunEnded marks the run as complete and writes the summary file.
func (l *Ledger) RunEnded(cancelled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	endedAt := nowMs()
	_ = l.appendEntryLocked(LedgerEntry{
		SchemaVersion: LedgerSchemaVersion,
		Type:          EntryRunEnd,
		TS:            endedAt,
		RunID:         l.runID,
		Status:        l.deriveRunStatus(cancelled),
	})

	summary := RunSummary{
		SchemaVersion: LedgerSchemaVersion,
		RunID:         l.runID,
		CycleID:       l.cycleID,
		Mode:          l.mode,
		WorkspaceMode: l.wsMode,
		StartedAt:     l.startedAt,
		EndedAt:       endedAt,
		Status:        l.deriveRunStatus(cancelled),
		Duration:      fmtDuration(time.Duration(endedAt-l.startedAt) * time.Millisecond),
		Cancelled:     cancelled,
		Steps:         l.steps,
	}
	var totalCost float64
	for _, s := range l.steps {
		totalCost += s.CostUSD
	}
	summary.TotalCostUSD = totalCost

	// Write summary file.
	if err := l.writeSummary(summary); err != nil {
		// Log only — ledger failure should not stop the run.
		fmt.Fprintf(os.Stderr, "ledger: write summary: %v\n", err)
	}
}

// LatestSummary returns the most recent run summary from disk, or nil.
func (l *Ledger) LatestSummary() (*RunSummary, error) {
	return l.readLatestSummary()
}

// LedgerDir returns the ledger directory path.
func (l *Ledger) LedgerDir() string { return l.ledgerDir }

// ---------------------------------------------------------------------------
// internals
// ---------------------------------------------------------------------------

func (l *Ledger) appendEntryLocked(e LedgerEntry) error {
	if err := os.MkdirAll(l.ledgerDir, 0o755); err != nil {
		return err
	}
	filename := filepath.Join(l.ledgerDir, l.runID+".jsonl")
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	return enc.Encode(e)
}

func (l *Ledger) writeSummary(s RunSummary) error {
	if err := os.MkdirAll(l.ledgerDir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(l.ledgerDir, l.runID+".summary.tmp")
	final := filepath.Join(l.ledgerDir, l.runID+".summary.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func (l *Ledger) readLatestSummary() (*RunSummary, error) {
	entries, err := os.ReadDir(l.ledgerDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Find the most recent .summary.json file by name (run IDs are timestamp-based).
	var latest string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".summary.json") && !e.IsDir() {
			if e.Name() > latest {
				latest = e.Name()
			}
		}
	}
	if latest == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filepath.Join(l.ledgerDir, latest))
	if err != nil {
		return nil, err
	}
	var s RunSummary
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (l *Ledger) deriveRunStatus(cancelled bool) string {
	if cancelled {
		return "cancelled"
	}
	if len(l.steps) == 0 {
		return "complete"
	}
	// Check the last step's status for halting.
	last := l.steps[len(l.steps)-1].FinalStatus
	switch last {
	case "failed", "blocked", "needs_review":
		return "halted"
	}
	return "complete"
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// UsageRecord mirrors runner.Usage but without import cycle.
type UsageRecord struct {
	InputTokens     int     `json:"input_tokens,omitempty"`
	OutputTokens    int     `json:"output_tokens,omitempty"`
	CacheReadTokens int     `json:"cache_read_tokens,omitempty"`
	Model           string  `json:"model,omitempty"`
	CostUSD         float64 `json:"cost_usd,omitempty"`
}

func truncateErr(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

func costIf(u *UsageRecord) float64 {
	if u == nil {
		return 0
	}
	return u.CostUSD
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
