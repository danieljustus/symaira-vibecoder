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
