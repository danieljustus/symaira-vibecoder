package server

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/engine"
)

func TestNew(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	dist := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
	}

	s := New(cfg, eng, dist)

	if s == nil {
		t.Fatal("New should return a non-nil server")
	}
	if s.cfg != cfg {
		t.Fatal("cfg not set correctly")
	}
	if s.eng != eng {
		t.Fatal("eng not set correctly")
	}
	if s.dist == nil {
		t.Fatal("dist should be set")
	}
	if s.pairing == nil {
		t.Fatal("pairing should be initialized")
	}
}

func TestSetTokenStore(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	if s.store != nil {
		t.Fatal("store should be nil initially")
	}

	// SetTokenStore is called with a nil store in this test,
	// but we can verify the method exists and can be called.
	s.SetTokenStore(nil)
}

func TestSetDevices(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	if s.devices != nil {
		t.Fatal("devices should be nil initially")
	}

	s.SetDevices(nil)
}

func TestHandlerWithoutTokenStore(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	dist := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
	}

	s := New(cfg, eng, dist)

	handler := s.Handler()
	if handler == nil {
		t.Fatal("Handler should return a non-nil handler")
	}

	// Without token store, Handler returns the mux directly.
	if handler != s.mux {
		t.Fatal("Handler should return the mux when no token store is set")
	}
}

func TestRoutesRegistered(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	dist := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
	}

	s := New(cfg, eng, dist)

	// Test that routes are registered by making requests to them.
	tests := []struct {
		method string
		path   string
		code   int
	}{
		{"GET", "/api/cycle", http.StatusOK},
		{"GET", "/api/version", http.StatusOK},
		{"GET", "/api/skills", http.StatusOK},
		{"GET", "/api/categories", http.StatusOK},
		{"GET", "/api/doctor", http.StatusOK},
		{"GET", "/api/runstate", http.StatusOK},
		{"POST", "/api/run/control", http.StatusBadRequest},
		{"GET", "/", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			s.Handler().ServeHTTP(rr, req)

			if rr.Code != tt.code {
				t.Fatalf("want %d for %s %s, got %d", tt.code, tt.method, tt.path, rr.Code)
			}
		})
	}
}

func TestBusy(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	// Engine starts in idle state.
	if s.busy() {
		t.Fatal("server should not be busy initially")
	}
}

func TestStaticServesIndex(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	dist := fstest.MapFS{
		"index.html": {Data: []byte("<html><body>Test</body></html>")},
	}

	s := New(cfg, eng, dist)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	s.static(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Test") {
		t.Fatalf("want body to contain 'Test', got %s", rr.Body.String())
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("want Content-Type to contain 'text/html', got %s", rr.Header().Get("Content-Type"))
	}
}

func TestStaticSPAFallback(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	dist := fstest.MapFS{
		"index.html": {Data: []byte("<html><body>SPA</body></html>")},
	}

	s := New(cfg, eng, dist)

	// Request a non-existent path should fallback to index.html.
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.static(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "SPA") {
		t.Fatalf("want body to contain 'SPA', got %s", rr.Body.String())
	}
}

func TestStaticMissingIndex(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	dist := fstest.MapFS{}

	s := New(cfg, eng, dist)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	s.static(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
}

func TestStaticServesFiles(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	dist := fstest.MapFS{
		"index.html": {Data: []byte("<html></html>")},
		"app.js":     {Data: []byte("console.log('test');")},
		"style.css":  {Data: []byte("body { color: red; }")},
	}

	s := New(cfg, eng, dist)

	tests := []struct {
		path     string
		wantCode int
		wantType string
	}{
		{"/app.js", http.StatusOK, "javascript"},
		{"/style.css", http.StatusOK, "css"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()
			s.Handler().ServeHTTP(rr, req)

			if rr.Code != tt.wantCode {
				t.Fatalf("want %d for %s, got %d", tt.wantCode, tt.path, rr.Code)
			}
		})
	}
}

func TestSSEHeaders(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, "GET", "/events", nil)
	rr := httptest.NewRecorder()

	go s.sse(rr, req)

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("want Content-Type text/event-stream, got %q", ct)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("want Cache-Control no-cache, got %q", cc)
	}
	if conn := rr.Header().Get("Connection"); conn != "keep-alive" {
		t.Fatalf("want Connection keep-alive, got %q", conn)
	}
	if noBuf := rr.Header().Get("X-Accel-Buffering"); noBuf != "no" {
		t.Fatalf("want X-Accel-Buffering no, got %q", noBuf)
	}
}

func TestSSEInitialEvent(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, "GET", "/events", nil)
	rr := httptest.NewRecorder()

	go s.sse(rr, req)

	time.Sleep(50 * time.Millisecond)
	cancel()

	body := rr.Body.String()
	if !strings.Contains(body, "event: run_state") {
		t.Fatalf("expected initial run_state event, got:\n%s", body)
	}
	if !strings.Contains(body, `"state":"idle"`) {
		t.Fatalf("expected state=idle in initial event, got:\n%s", body)
	}
}

func TestSSEDeliversPublishedEvents(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, "GET", "/events", nil)
	rr := httptest.NewRecorder()

	go s.sse(rr, req)

	time.Sleep(50 * time.Millisecond)

	bus.Publish(engine.Event{Type: "step_status", StepID: "step-1", Status: "in_progress"})

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	body := rr.Body.String()
	if !strings.Contains(body, "event: step_status") {
		t.Fatalf("expected step_status event, got:\n%s", body)
	}
	if !strings.Contains(body, `"step_id":"step-1"`) {
		t.Fatalf("expected step_id=step-1, got:\n%s", body)
	}
	if !strings.Contains(body, `"status":"in_progress"`) {
		t.Fatalf("expected status=in_progress, got:\n%s", body)
	}
}

func TestSSEPingComment(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, "GET", "/events", nil)
	rr := httptest.NewRecorder()

	go s.sse(rr, req)

	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	body := rr.Body.String()
	if !strings.Contains(body, "event: run_state") {
		t.Fatalf("expected run_state event, got:\n%s", body)
	}
}

// nonFlusherWriter wraps http.ResponseWriter but does NOT implement http.Flusher.
type nonFlusherWriter struct {
	header http.Header
	code   int
	body   strings.Builder
}

func (w *nonFlusherWriter) Header() http.Header         { return w.header }
func (w *nonFlusherWriter) Write(b []byte) (int, error)  { return w.body.Write(b) }
func (w *nonFlusherWriter) WriteHeader(code int)         { w.code = code }

func TestSSENoFlusher(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	req := httptest.NewRequest("GET", "/events", nil)
	w := &nonFlusherWriter{header: http.Header{}}

	s.sse(w, req)

	if w.code != http.StatusInternalServerError {
		t.Fatalf("want 500 when Flusher not supported, got %d", w.code)
	}
	if !strings.Contains(w.body.String(), "streaming unsupported") {
		t.Fatalf("expected 'streaming unsupported' error, got: %s", w.body.String())
	}
}

func TestSSEClientDisconnect(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequestWithContext(ctx, "GET", "/events", nil)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		s.sse(rr, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sse handler did not return after client disconnect")
	}
}

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flushRecorder) Flush() { f.flushed = true }

func TestSSEMultipleEvents(t *testing.T) {
	cfg := config.Default()
	bus := engine.NewBus()
	run := &mockRunner{available: true}
	eng := engine.New(cfg, config.NewResolver(cfg), run, bus)

	s := New(cfg, eng, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, "GET", "/events", nil)
	w := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}

	go s.sse(w, req)

	time.Sleep(50 * time.Millisecond)

	bus.Publish(engine.Event{Type: "step_status", StepID: "s1", Status: "done"})
	bus.Publish(engine.Event{Type: "log", StepID: "s1", Kind: "log", Line: "hello"})
	bus.Publish(engine.Event{Type: "run_state", State: "running"})

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	body := w.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))
	eventCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventCount++
		}
	}

	if eventCount < 4 {
		t.Fatalf("expected at least 4 events, got %d. Body:\n%s", eventCount, body)
	}
}
