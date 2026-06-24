package devices

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

type Device struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TokenHash string `json:"token_hash"`
	Salt      string `json:"salt"`
	Created   string `json:"created"`
	LastSeen  string `json:"last_seen"`
}

type Registry struct {
	mu      sync.RWMutex
	path    string
	devices []Device
}

func Open() (*Registry, error) {
	path := filepath.Join(config.DataDir(), "devices.json")
	r := &Registry{path: path}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) load() error {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("devices: load: %w", err)
	}
	return json.Unmarshal(data, &r.devices)
}

func (r *Registry) save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r.devices, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

func (r *Registry) Authenticate(token string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for i := range r.devices {
		if verifyToken(token, r.devices[i].Salt, r.devices[i].TokenHash) {
			now := time.Now().UTC().Format(time.RFC3339)
			r.devices[i].LastSeen = now
			go r.save()
			return true
		}
	}
	return false
}

func (r *Registry) Add(name, token string) (id, tokenOut string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	salt, err := randomHex(16)
	if err != nil {
		return "", "", fmt.Errorf("devices: generate salt: %w", err)
	}
	id, err = randomHex(8)
	if err != nil {
		return "", "", fmt.Errorf("devices: generate id: %w", err)
	}
	tokenOut = token
	if tokenOut == "" {
		t, err := randomHex(24)
		if err != nil {
			return "", "", fmt.Errorf("devices: generate token: %w", err)
		}
		tokenOut = t
	}

	r.devices = append(r.devices, Device{
		ID:        id,
		Name:      name,
		TokenHash: hashToken(tokenOut, salt),
		Salt:      salt,
		Created:   time.Now().UTC().Format(time.RFC3339),
		LastSeen:  time.Now().UTC().Format(time.RFC3339),
	})

	if err := r.save(); err != nil {
		return "", "", err
	}
	return id, tokenOut, nil
}

func (r *Registry) Delete(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.devices {
		if r.devices[i].ID == id {
			r.devices = append(r.devices[:i], r.devices[i+1:]...)
			_ = r.save()
			return true
		}
	}
	return false
}

func (r *Registry) List() []Device {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Device, len(r.devices))
	copy(out, r.devices)
	return out
}

func hashToken(token, salt string) string {
	h := sha256.Sum256([]byte(salt + token))
	return hex.EncodeToString(h[:])
}

func verifyToken(token, salt, expected string) bool {
	return hashToken(token, salt) == expected
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
