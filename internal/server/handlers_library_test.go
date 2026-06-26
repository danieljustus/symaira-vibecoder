package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetLibraryIndex(t *testing.T) {
	// Start a local HTTP server that simulates the upstream template index.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries := []LibraryEntry{
			{
				ID:          "go-tdd",
				Name:        "Go TDD Cycle",
				Author:      "alice",
				Tags:        []string{"go", "tdd"},
				Description: "A test-driven Go development cycle.",
				URL:         "https://raw.githubusercontent.com/danieljustus/symvibe-templates/main/templates/go-tdd.json",
			},
			{
				ID:          "python-ml",
				Name:        "Python ML Pipeline",
				Author:      "bob",
				Tags:        []string{"python", "ml"},
				Description: "Machine-learning workflow cycle.",
				URL:         "https://raw.githubusercontent.com/danieljustus/symvibe-templates/main/templates/python-ml.json",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	}))
	defer upstream.Close()

	s := newTestServer(t, true)
	s.cfg.Server.LibraryIndexURL = upstream.URL
	// Reset the cache so the new URL is used.
	s.libCache = newLibraryCache(defaultLibraryTTL)

	req := httptest.NewRequest("GET", "/api/library/index", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var entries []LibraryEntry
	if err := json.Unmarshal(rr.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].ID != "go-tdd" || entries[1].ID != "python-ml" {
		t.Errorf("unexpected entries: %+v", entries)
	}

	// Second request should be served from cache (upstream would not be called again).
	req2 := httptest.NewRequest("GET", "/api/library/index", nil)
	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 from cache, got %d", rr2.Code)
	}
	var cached []LibraryEntry
	if err := json.Unmarshal(rr2.Body.Bytes(), &cached); err != nil {
		t.Fatalf("failed to decode cached response: %v", err)
	}
	if len(cached) != 2 {
		t.Errorf("cache returned wrong count: %d", len(cached))
	}
}

func TestGetLibraryIndexBadGateway(t *testing.T) {
	// Upstream returns 500 — handler should propagate a 502.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer upstream.Close()

	s := newTestServer(t, true)
	s.cfg.Server.LibraryIndexURL = upstream.URL
	s.libCache = newLibraryCache(defaultLibraryTTL)

	req := httptest.NewRequest("GET", "/api/library/index", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected 502 for upstream error, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetLibraryIndexExpiredCache(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]LibraryEntry{{ID: "entry", Name: "Entry", URL: "http://example.com/entry.json"}})
	}))
	defer upstream.Close()

	s := newTestServer(t, true)
	s.cfg.Server.LibraryIndexURL = upstream.URL
	// Very short TTL so the cache expires after the first fetch.
	s.libCache = newLibraryCache(time.Millisecond)

	req := httptest.NewRequest("GET", "/api/library/index", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rr.Code)
	}

	time.Sleep(5 * time.Millisecond) // let the cache expire

	req2 := httptest.NewRequest("GET", "/api/library/index", nil)
	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", rr2.Code)
	}

	if calls != 2 {
		t.Errorf("expected upstream to be called twice (cache expiry), got %d calls", calls)
	}
}
