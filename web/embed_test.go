package web

import (
	"io/fs"
	"testing"
)

func TestDistFS(t *testing.T) {
	dist := DistFS()
	entries, err := fs.ReadDir(dist, ".")
	if err != nil {
		t.Fatalf("DistFS() read dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("DistFS() has no entries")
	}
	// The committed dependency-free board must contain at least an index.html.
	if _, err := fs.ReadFile(dist, "index.html"); err != nil {
		t.Fatalf("DistFS() missing index.html: %v", err)
	}
}
