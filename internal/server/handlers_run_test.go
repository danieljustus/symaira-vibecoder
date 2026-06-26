package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/engine"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

// mockRunner implements runner.Runner for testing.
type mockRunner struct {
	available bool
	info      runner.Info
	runStep   func(context.Context, runner.StepRequest) (<-chan runner.RunEvent, error)
}

func (r *mockRunner) Name() string { return "test" }

func (r *mockRunner) Available(_ context.Context) (bool, runner.Info) {
	return r.available, r.info
}

func (r *mockRunner) RunStep(ctx context.Context, req runner.StepRequest) (<-chan runner.RunEvent, error) {
	if r.runStep != nil {
		return r.runStep(ctx, req)
	}
	return nil, errors.New("not implemented")
}

// newTestServer creates a Server with a real engine for testing.
func newTestServer(t *testing.T, available bool) *Server {
	t.Helper()

	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "test-cycle"

	run := &mockRunner{
		available: available,
		info:      runner.Info{Name: "test", Detail: "test runner"},
	}
	bus := engine.NewBus()
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	return New(cfg, eng, nil)
}

func TestGetRunState(t *testing.T) {
	s := newTestServer(t, true)

	req := httptest.NewRequest("GET", "/api/runstate", nil)
	rr := httptest.NewRecorder()
	s.getRunState(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "idle") {
		t.Fatalf("want body to contain 'idle', got %s", rr.Body.String())
	}
}

func TestRequireRunnableAvailable(t *testing.T) {
	s := newTestServer(t, true)

	req := httptest.NewRequest("POST", "/api/run", nil)
	rr := httptest.NewRecorder()
	ok := s.requireRunnable(rr, req)

	if !ok {
		t.Fatal("requireRunnable should return true when available")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestRequireRunnableUnavailable(t *testing.T) {
	s := newTestServer(t, false)

	req := httptest.NewRequest("POST", "/api/run", nil)
	rr := httptest.NewRecorder()
	ok := s.requireRunnable(rr, req)

	if ok {
		t.Fatal("requireRunnable should return false when unavailable")
	}
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "runner unavailable") {
		t.Fatalf("want body to contain 'runner unavailable', got %s", rr.Body.String())
	}
}

func TestRunCycleSuccess(t *testing.T) {
	s := newTestServer(t, true)

	req := httptest.NewRequest("POST", "/api/run", nil)
	rr := httptest.NewRecorder()
	s.runCycle(rr, req)

	// The engine starts the run asynchronously and returns 202 Accepted.
	// The actual error (missing cycle) is published via the event bus.
	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", rr.Code)
	}
}

func TestRunCycleUnavailable(t *testing.T) {
	s := newTestServer(t, false)

	req := httptest.NewRequest("POST", "/api/run", nil)
	rr := httptest.NewRecorder()
	s.runCycle(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rr.Code)
	}
}

func TestRunStepSuccess(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"step_id": "step-1"}`
	req := httptest.NewRequest("POST", "/api/run/step", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.runStep(rr, req)

	// The engine starts the run asynchronously and returns 202 Accepted.
	// The actual error (missing cycle) is published via the event bus.
	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", rr.Code)
	}
}

func TestRunStepMissingStepID(t *testing.T) {
	s := newTestServer(t, true)

	body := `{}`
	req := httptest.NewRequest("POST", "/api/run/step", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.runStep(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "step_id required") {
		t.Fatalf("want body to contain 'step_id required', got %s", rr.Body.String())
	}
}

func TestRunStepInvalidJSON(t *testing.T) {
	s := newTestServer(t, true)

	body := `invalid json`
	req := httptest.NewRequest("POST", "/api/run/step", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.runStep(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
}

func TestRunStepUnavailable(t *testing.T) {
	s := newTestServer(t, false)

	body := `{"step_id": "step-1"}`
	req := httptest.NewRequest("POST", "/api/run/step", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.runStep(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rr.Code)
	}
}

func TestWriteRunStartSuccess(t *testing.T) {
	s := &Server{}
	rr := httptest.NewRecorder()
	s.writeRunStart(rr, "run-789", nil)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "run-789") {
		t.Fatalf("want body to contain 'run-789', got %s", rr.Body.String())
	}
}

func TestWriteRunStartBusy(t *testing.T) {
	s := &Server{}
	rr := httptest.NewRecorder()
	s.writeRunStart(rr, "", engine.ErrBusy)

	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rr.Code)
	}
}

func TestWriteRunStartError(t *testing.T) {
	s := &Server{}
	rr := httptest.NewRecorder()
	s.writeRunStart(rr, "", context.DeadlineExceeded)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rr.Code)
	}
}

func TestRunControlCancel(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"action": "cancel"}`
	req := httptest.NewRequest("POST", "/api/run/control", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.runControl(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestRunControlPause(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"action": "pause"}`
	req := httptest.NewRequest("POST", "/api/run/control", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.runControl(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestRunControlResume(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"action": "resume"}`
	req := httptest.NewRequest("POST", "/api/run/control", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.runControl(rr, req)

	// Resume will try to start the engine, which may fail depending on state.
	// We're testing that the handler processes the action correctly.
	if rr.Code != http.StatusConflict && rr.Code != http.StatusAccepted {
		t.Fatalf("want 409 or 202, got %d", rr.Code)
	}
}

func TestRunControlUnknownAction(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"action": "invalid"}`
	req := httptest.NewRequest("POST", "/api/run/control", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.runControl(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "unknown action") {
		t.Fatalf("want body to contain 'unknown action', got %s", rr.Body.String())
	}
}

func TestRunControlInvalidJSON(t *testing.T) {
	s := newTestServer(t, true)

	body := `invalid`
	req := httptest.NewRequest("POST", "/api/run/control", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.runControl(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
}
