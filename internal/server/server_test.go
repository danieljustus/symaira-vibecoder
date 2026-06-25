package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

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
