package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/engine"
)

func newLogsTestServer(t *testing.T) (*Server, *engine.Bus) {
	t.Helper()
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)
	return New(cfg, eng, nil), bus
}

// TestGetLogsEmpty verifies GET /api/logs returns an empty (non-null) entry
// list when nothing has been published yet.
func TestGetLogsEmpty(t *testing.T) {
	s, _ := newLogsTestServer(t)

	req := httptest.NewRequest("GET", "/api/logs", nil)
	rr := httptest.NewRecorder()
	s.getLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var resp logsResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Entries == nil {
		t.Fatal("want entries to be a non-null empty array")
	}
	if len(resp.Entries) != 0 {
		t.Fatalf("want 0 entries, got %d", len(resp.Entries))
	}
}

// TestGetLogsReturnsBuffer verifies published log/error events are returned
// oldest-first, and transient events are not.
func TestGetLogsReturnsBuffer(t *testing.T) {
	s, bus := newLogsTestServer(t)

	bus.Publish(engine.Event{Type: "log", RunID: "run_x", StepID: "s1", Kind: "log", Line: "first"})
	bus.Publish(engine.Event{Type: "run_state", State: "running"})
	bus.Publish(engine.Event{Type: "error", RunID: "run_x", Line: "second"})

	req := httptest.NewRequest("GET", "/api/logs", nil)
	rr := httptest.NewRecorder()
	s.getLogs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var resp logsResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("want 2 entries, got %d: %s", len(resp.Entries), rr.Body.String())
	}
	if resp.Entries[0].Line != "first" || resp.Entries[0].Type != "log" {
		t.Fatalf("want first entry log/first, got %+v", resp.Entries[0])
	}
	if resp.Entries[1].Line != "second" || resp.Entries[1].Type != "error" {
		t.Fatalf("want second entry error/second, got %+v", resp.Entries[1])
	}
	if resp.Entries[0].TS == 0 || resp.Entries[1].TS == 0 {
		t.Fatal("want non-zero ts on replayed entries (clients merge by ts)")
	}
}

// TestGetLogsRunID verifies the handler reports the engine's current run id.
// With no run active the engine is idle, so run_id must be empty.
func TestGetLogsRunID(t *testing.T) {
	s, bus := newLogsTestServer(t)
	bus.Publish(engine.Event{Type: "log", Line: "x"})

	req := httptest.NewRequest("GET", "/api/logs", nil)
	rr := httptest.NewRecorder()
	s.getLogs(rr, req)

	var resp logsResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.RunID != "" {
		t.Fatalf("want empty run_id while idle, got %q", resp.RunID)
	}
	if !json.Valid(rr.Body.Bytes()) {
		t.Fatal("response must be valid JSON")
	}
}
