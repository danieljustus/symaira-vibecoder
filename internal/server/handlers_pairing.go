package server

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/devices"
	"github.com/danieljustus/symaira-vibecoder/internal/server/tlsutil"
)

type pairingCode struct {
	Name      string
	CreatedAt time.Time
}

type pairingStore struct {
	mu    sync.Mutex
	codes map[string]*pairingCode
}

func newPairingStore() *pairingStore {
	return &pairingStore{codes: make(map[string]*pairingCode)}
}

func (ps *pairingStore) create(name string) string {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	code := generateCode(6)
	ps.codes[code] = &pairingCode{
		Name:      name,
		CreatedAt: time.Now(),
	}
	return code
}

func (ps *pairingStore) consume(code string) (*pairingCode, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	pc, ok := ps.codes[code]
	if !ok {
		return nil, false
	}
	delete(ps.codes, code)
	if time.Since(pc.CreatedAt) > 120*time.Second {
		return nil, false
	}
	return pc, true
}

func generateCode(n int) string {
	const charset = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

func (s *Server) pairStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "device"
	}
	code := s.pairing.create(body.Name)
	writeOK(w, map[string]string{"code": code, "ttl": "120s"})
}

func (s *Server) pairComplete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Code == "" {
		writeErr(w, http.StatusBadRequest, "code required")
		return
	}
	pc, ok := s.pairing.consume(body.Code)
	if !ok {
		writeErr(w, http.StatusGone, "invalid or expired pairing code")
		return
	}
	name := body.Name
	if name == "" {
		name = pc.Name
	}
	id, token, err := s.devices.Add(name, "")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"id": id, "name": name, "token": token})
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.devices.List())
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.devices.Delete(id) {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

func (s *Server) pairQR(w http.ResponseWriter, r *http.Request) {
	fp := s.getFingerprint()
	code := s.pairing.create("qr-device")
	payload := buildQRPayload(s.cfg, fp, code)
	writeOK(w, map[string]string{"payload": payload, "code": code})
}

func (s *Server) getFingerprint() string {
	hostname, _ := os.Hostname()
	pair, err := tlsutil.EnsureCert(config.DataDir(), hostname)
	if err != nil {
		return ""
	}
	return pair.Fingerprint
}

func buildQRPayload(cfg *config.Config, fp, code string) string {
	host := cfg.Server.Host
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	name, _ := os.Hostname()
	if name == "" {
		name = "symvibe"
	}
	return fmt.Sprintf("symvibe://pair?n=%s&p=%d&h=%s&fp=%s&c=%s",
		name, cfg.Server.Port, host, fp, code)
}

var _ = (*devices.Registry)(nil)
