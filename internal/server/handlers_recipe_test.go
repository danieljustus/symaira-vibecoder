package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/engine"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

func newRecipeTestServer(t *testing.T, runStepFn func(context.Context, runner.StepRequest) (<-chan runner.RunEvent, error)) *Server {
	t.Helper()

	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	cfg.Defaults.Cycle = "test-cycle"

	run := &mockRunner{
		available: true,
		info:      runner.Info{Name: "test", Detail: "test runner"},
		runStep:   runStepFn,
	}
	bus := engine.NewBus()
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	return New(cfg, eng, nil)
}

func TestRecipeRunSuccess(t *testing.T) {
	s := newRecipeTestServer(t, func(_ context.Context, _ runner.StepRequest) (<-chan runner.RunEvent, error) {
		ch := make(chan runner.RunEvent, 2)
		ch <- runner.RunEvent{Kind: runner.EventDone, Text: "completed"}
		close(ch)
		return ch, nil
	})

	body := `{"schema_version":"1","prompt":"write a test","workspace":"/tmp"}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"done"`) {
		t.Fatalf("want status=done in body, got %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"schema_version":"1"`) {
		t.Fatalf("want schema_version=1 in body, got %s", rr.Body.String())
	}
}

func TestRecipeRunMissingPrompt(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"schema_version":"1","workspace":"/tmp"}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "prompt") {
		t.Fatalf("want error about prompt, got %s", rr.Body.String())
	}
}

func TestRecipeRunMissingWorkspace(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"schema_version":"1","prompt":"do something"}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRecipeRunInvalidJSON(t *testing.T) {
	s := newTestServer(t, true)

	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRecipeRunWrongSchema(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"schema_version":"99","prompt":"x","workspace":"/tmp"}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "schema_version") {
		t.Fatalf("want error about schema_version, got %s", rr.Body.String())
	}
}

func TestRecipeRunBackendUnavailable(t *testing.T) {
	s := newTestServer(t, false)

	body := `{"schema_version":"1","prompt":"do something","workspace":"/tmp"}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRecipeRunWithToolAllowList(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"schema_version":"1","prompt":"read files","workspace":"/tmp","tool_allow_list":["read_file","git_status"]}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "read_file") {
		t.Fatalf("want tool_allow_list in body, got %s", rr.Body.String())
	}
}

func TestRecipeRunWriteCapNone(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"schema_version":"1","prompt":"read only","workspace":"/tmp","write_cap":"none"}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"write_cap":"none"`) {
		t.Fatalf("want write_cap=none in body, got %s", rr.Body.String())
	}
}

func TestRecipeRunReviewMode(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"schema_version":"1","prompt":"make changes","workspace":"/tmp","review_mode":true}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRecipeRunModelAndAgent(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"schema_version":"1","prompt":"use specific model","workspace":"/tmp","model":"anthropic/claude","agent":"build","variant":"high"}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRecipeRunBackendRunStepError(t *testing.T) {
	s := newTestServer(t, true)

	body := `{"schema_version":"1","prompt":"will fail","workspace":"/tmp"}`
	req := httptest.NewRequest("POST", "/api/recipe/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.recipeRun(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 (error surfaced in result), got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"failed"`) {
		t.Fatalf("want status=failed in body, got %s", rr.Body.String())
	}
}
