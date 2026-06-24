package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPairStartAndComplete(t *testing.T) {
	ps := newPairingStore()
	code := ps.create("test-device")
	if code == "" {
		t.Fatal("code must not be empty")
	}
	pc, ok := ps.consume(code)
	if !ok {
		t.Fatal("consume should succeed")
	}
	if pc.Name != "test-device" {
		t.Fatalf("want name test-device, got %s", pc.Name)
	}
	if _, ok := ps.consume(code); ok {
		t.Fatal("code should be single-use")
	}
}

func TestPairCodeExpires(t *testing.T) {
	ps := newPairingStore()
	ps.codes["expired"] = &pairingCode{
		Name:      "old",
		CreatedAt: time.Now().Add(-200 * time.Second),
	}
	if _, ok := ps.consume("expired"); ok {
		t.Fatal("expired code should be rejected")
	}
}

func TestPairCompleteInvalidCode(t *testing.T) {
	s := &Server{pairing: newPairingStore()}
	body := `{"code":"INVALID","name":"test"}`
	req := httptest.NewRequest("POST", "/api/pair/complete", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.pairComplete(rr, req)
	if rr.Code != http.StatusGone {
		t.Fatalf("want 410, got %d", rr.Code)
	}
}
