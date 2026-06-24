package devices

import (
	"path/filepath"
	"testing"
)

func TestRegistryAddAndAuthenticate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devices.json")
	r := &Registry{path: path}

	id, token, err := r.Add("test-device", "")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id == "" || token == "" {
		t.Fatal("id and token must not be empty")
	}
	if !r.Authenticate(token) {
		t.Fatal("Authenticate should succeed with correct token")
	}
	if r.Authenticate("wrong-token") {
		t.Fatal("Authenticate should fail with wrong token")
	}
}

func TestRegistryDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devices.json")
	r := &Registry{path: path}

	id, token, _ := r.Add("device-1", "")
	if !r.Delete(id) {
		t.Fatal("Delete should return true")
	}
	if r.Authenticate(token) {
		t.Fatal("Authenticate should fail after delete")
	}
	if r.Delete("nonexistent") {
		t.Fatal("Delete nonexistent should return false")
	}
}

func TestRegistryList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devices.json")
	r := &Registry{path: path}

	r.Add("d1", "")
	r.Add("d2", "")
	list := r.List()
	if len(list) != 2 {
		t.Fatalf("want 2 devices, got %d", len(list))
	}
}

func TestRegistryPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devices.json")
	r1 := &Registry{path: path}
	id, token, _ := r1.Add("persistent-device", "")

	r2 := &Registry{path: path}
	if err := r2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !r2.Authenticate(token) {
		t.Fatal("token should survive reload")
	}
	list := r2.List()
	if len(list) != 1 || list[0].ID != id {
		t.Fatalf("device not persisted correctly: %+v", list)
	}
}

func TestRegistryNoFileIsGraceful(t *testing.T) {
	r := &Registry{path: filepath.Join(t.TempDir(), "nonexistent.json")}
	if err := r.load(); err != nil {
		t.Fatalf("missing file should be graceful: %v", err)
	}
}

func TestRegistryCustomToken(t *testing.T) {
	dir := t.TempDir()
	r := &Registry{path: filepath.Join(dir, "devices.json")}

	id, _, err := r.Add("custom", "my-custom-token")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !r.Authenticate("my-custom-token") {
		t.Fatal("custom token should authenticate")
	}
	_ = id
}

func TestRegistryAddGeneratesRandomToken(t *testing.T) {
	dir := t.TempDir()
	r := &Registry{path: filepath.Join(dir, "devices.json")}

	_, tok1, _ := r.Add("d1", "")
	_, tok2, _ := r.Add("d2", "")
	if tok1 == tok2 {
		t.Fatal("auto-generated tokens must differ")
	}
}

func TestRegistryTokenHashIsSalted(t *testing.T) {
	dir := t.TempDir()
	r := &Registry{path: filepath.Join(dir, "devices.json")}

	_, _, _ = r.Add("d1", "same-password")
	list := r.List()
	if len(list) != 1 {
		t.Fatal("expected 1 device")
	}
	list2 := &Registry{path: filepath.Join(t.TempDir(), "devices.json")}
	list2.Add("d2", "same-password")
	list2Devices := list2.List()
	if list[0].TokenHash == list2Devices[0].TokenHash {
		t.Fatal("same password with different salts must produce different hashes")
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	r := &Registry{path: filepath.Join(dir, "devices.json")}
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			r.Add("d", "")
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if len(r.List()) != 10 {
		t.Fatalf("want 10, got %d", len(r.List()))
	}
}
