package engine

import (
	"fmt"
	"testing"
)

// TestBusLogRingRetainsLogAndError verifies that log and error events published
// on the bus land in the replay buffer in publish order.
func TestBusLogRingRetainsLogAndError(t *testing.T) {
	b := NewBus()
	b.Publish(Event{Type: "log", RunID: "run_1", StepID: "s1", Kind: "log", Line: "hello"})
	b.Publish(Event{Type: "error", RunID: "run_1", Line: "boom"})

	got := b.Logs()
	if len(got) != 2 {
		t.Fatalf("want 2 buffered events, got %d", len(got))
	}
	if got[0].Line != "hello" || got[0].Type != "log" {
		t.Fatalf("want first event hello/log, got %+v", got[0])
	}
	if got[1].Line != "boom" || got[1].Type != "error" {
		t.Fatalf("want second event boom/error, got %+v", got[1])
	}
}

// TestBusLogRingSkipsTransientEvents verifies that non-log events (run_state,
// step_status, board) are never retained — they are re-derived or re-primed on
// connect and would only waste the bounded buffer.
func TestBusLogRingSkipsTransientEvents(t *testing.T) {
	b := NewBus()
	b.Publish(Event{Type: "run_state", State: "running"})
	b.Publish(Event{Type: "step_status", StepID: "s1", Status: "done"})
	b.Publish(Event{Type: "board"})
	b.Publish(Event{Type: "log", Line: "only me"})

	got := b.Logs()
	if len(got) != 1 {
		t.Fatalf("want only the log event buffered, got %d: %+v", len(got), got)
	}
	if got[0].Line != "only me" {
		t.Fatalf("want 'only me', got %+v", got[0])
	}
}

// TestBusLogRingCapacity verifies the buffer is bounded at 500 entries and
// evicts oldest-first, matching ActivityStore.maxLines / the board's cap.
func TestBusLogRingCapacity(t *testing.T) {
	b := NewBus()
	const n = 600
	for i := 0; i < n; i++ {
		b.Publish(Event{Type: "log", Line: fmt.Sprintf("line-%d", i)})
	}

	got := b.Logs()
	if len(got) != logRingCapacity {
		t.Fatalf("want %d buffered events, got %d", logRingCapacity, len(got))
	}
	if got[0].Line != "line-100" {
		t.Fatalf("want oldest retained event line-100, got %q", got[0].Line)
	}
	if got[len(got)-1].Line != "line-599" {
		t.Fatalf("want newest retained event line-599, got %q", got[len(got)-1].Line)
	}
}

// TestBusLogRingAssignsTS verifies buffered events carry the bus-assigned
// timestamp so clients can merge by ts.
func TestBusLogRingAssignsTS(t *testing.T) {
	b := NewBus()
	b.Publish(Event{Type: "log", Line: "timed"})

	got := b.Logs()
	if len(got) != 1 {
		t.Fatalf("want 1 buffered event, got %d", len(got))
	}
	if got[0].TS == 0 {
		t.Fatal("want non-zero ts on buffered event")
	}
}

// TestBusLogsSnapshotDetached verifies the returned slice is a copy: mutating
// it must not corrupt the ring.
func TestBusLogsSnapshotDetached(t *testing.T) {
	b := NewBus()
	b.Publish(Event{Type: "log", Line: "a"})
	b.Publish(Event{Type: "log", Line: "b"})

	got := b.Logs()
	got[0].Line = "tampered"
	got[1].Line = "tampered"

	again := b.Logs()
	if again[0].Line != "a" || again[1].Line != "b" {
		t.Fatalf("snapshot must be detached, got %+v", again)
	}
}
