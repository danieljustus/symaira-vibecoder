package tlsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCertGeneratesAndLoads(t *testing.T) {
	dir := t.TempDir()
	pair, err := EnsureCert(dir, "testhost")
	if err != nil {
		t.Fatalf("EnsureCert: %v", err)
	}
	if pair.Fingerprint == "" {
		t.Fatal("fingerprint must not be empty")
	}
	if _, err := os.Stat(pair.CertPath); err != nil {
		t.Fatalf("cert not created: %v", err)
	}
	if _, err := os.Stat(pair.KeyPath); err != nil {
		t.Fatalf("key not created: %v", err)
	}

	pair2, err := EnsureCert(dir, "testhost")
	if err != nil {
		t.Fatalf("EnsureCert (reuse): %v", err)
	}
	if pair.Fingerprint != pair2.Fingerprint {
		t.Fatalf("fingerprint mismatch: %s != %s", pair.Fingerprint, pair2.Fingerprint)
	}
}

func TestEnsureCertWithIP(t *testing.T) {
	dir := t.TempDir()
	pair, err := EnsureCert(dir, "192.168.1.100")
	if err != nil {
		t.Fatalf("EnsureCert with IP: %v", err)
	}
	if pair.Fingerprint == "" {
		t.Fatal("fingerprint must not be empty")
	}
}

func TestDir(t *testing.T) {
	got := Dir("/data")
	want := filepath.Join("/data", "tls")
	if got != want {
		t.Fatalf("Dir: got %q, want %q", got, want)
	}
}
