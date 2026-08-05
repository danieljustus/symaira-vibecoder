package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type fixedStore struct{ ok bool }

func (f fixedStore) Authenticate(token string) bool { return f.ok }

func TestMiddlewareBypassLoopback(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := Middleware(next, fixedStore{false}, true)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Fatal("loopback bypass should pass through")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestMiddlewareRejectsNoToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	h := Middleware(next, fixedStore{false}, false)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestMiddlewareAcceptsBearerHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	store := &mapStore{tokens: map[string]bool{"secret123": true}}
	h := Middleware(next, store, false)

	req := httptest.NewRequest("GET", "/api/cycle", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	req.RemoteAddr = "10.0.0.1:12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Fatal("valid token should pass through")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestMiddlewareAcceptsTokenQueryParam(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	store := &mapStore{tokens: map[string]bool{"qtoken": true}}
	h := Middleware(next, store, false)

	req := httptest.NewRequest("GET", "/events?token=qtoken", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !called {
		t.Fatal("valid query token should pass through")
	}
}

func TestMiddlewareRejectsBadToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	store := &mapStore{tokens: map[string]bool{"real": true}}
	h := Middleware(next, store, false)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	req.RemoteAddr = "10.0.0.1:12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestOriginGuard(t *testing.T) {
	tests := []struct {
		name         string
		allowOrigin  string
		secFetchSite string
		origin       string
		host         string
		wantCode     int
	}{
		// Requests without an Origin header (curl, SwiftUI client, recipe/MCP
		// callers) pass through unchanged.
		{name: "no origin passes", host: "127.0.0.1:4317", wantCode: http.StatusOK},
		// Browser-labelled cross-site requests are rejected outright.
		{name: "cross-site sec-fetch-site rejected", secFetchSite: "cross-site", host: "127.0.0.1:4317", wantCode: http.StatusForbidden},
		{name: "same-origin sec-fetch-site passes", secFetchSite: "same-origin", origin: "http://127.0.0.1:4317", host: "127.0.0.1:4317", wantCode: http.StatusOK},
		{name: "none sec-fetch-site passes", secFetchSite: "none", host: "127.0.0.1:4317", wantCode: http.StatusOK},
		// Origin header present, Host resolves to loopback: hostnames must match.
		{name: "mismatched origin vs loopback host rejected", origin: "https://evil.example", host: "127.0.0.1:4317", wantCode: http.StatusForbidden},
		{name: "matching origin on loopback passes", origin: "http://127.0.0.1:4317", host: "127.0.0.1:4317", wantCode: http.StatusOK},
		{name: "matching origin on localhost passes", origin: "http://localhost:4317", host: "localhost:4317", wantCode: http.StatusOK},
		{name: "localhost vs 127.0.0.1 hostname mismatch rejected", origin: "http://localhost:4317", host: "127.0.0.1:4317", wantCode: http.StatusForbidden},
		// Non-loopback Hosts (lan/relay behind the token gate) are untouched.
		{name: "mismatched origin on non-loopback host passes", origin: "https://evil.example", host: "192.168.1.5:4317", wantCode: http.StatusOK},
		// allow_origin escape hatch for embedding the board elsewhere.
		{name: "allow_origin match passes", allowOrigin: "https://app.example.com", secFetchSite: "cross-site", origin: "https://app.example.com", host: "127.0.0.1:4317", wantCode: http.StatusOK},
		{name: "allow_origin mismatch rejected", allowOrigin: "https://app.example.com", origin: "https://other.example", host: "127.0.0.1:4317", wantCode: http.StatusForbidden},
		{name: "allow_origin wildcard passes", allowOrigin: "*", secFetchSite: "cross-site", origin: "https://anything.example", host: "127.0.0.1:4317", wantCode: http.StatusOK},
		{name: "allow_origin host comparison is case-insensitive", allowOrigin: "https://app.example.com", origin: "HTTPS://APP.EXAMPLE.COM", host: "127.0.0.1:4317", wantCode: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			h := OriginGuard(next, tt.allowOrigin)

			req := httptest.NewRequest("POST", "/api/run", nil)
			req.Host = tt.host
			if tt.secFetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetchSite)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tt.wantCode {
				t.Fatalf("want %d, got %d", tt.wantCode, rr.Code)
			}
			if tt.wantCode == http.StatusOK && !called {
				t.Fatal("next handler should be called for pass-through requests")
			}
			if tt.wantCode == http.StatusForbidden && called {
				t.Fatal("next handler must not be called for rejected requests")
			}
		})
	}
}

func TestMiddlewareNoBypassNonLoopback(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called without token")
	})
	h := Middleware(next, fixedStore{false}, true)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.10:5000"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 for non-loopback without token, got %d", rr.Code)
	}
}

type mapStore struct {
	tokens map[string]bool
}

func (m *mapStore) Authenticate(token string) bool { return m.tokens[token] }
