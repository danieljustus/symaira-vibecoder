package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

func TestNoopReviewProvider(t *testing.T) {
	p := NoopReviewProvider{}
	if p.Name() != "none" {
		t.Fatalf("name = %q, want none", p.Name())
	}
	ctx := context.Background()
	rc, err := p.Provide(ctx, "/tmp", "step-1")
	if err != nil {
		t.Fatalf("Provide: %v", err)
	}
	if rc != nil {
		t.Fatalf("expected nil context from noop, got %+v", rc)
	}
}

func TestGitReviewProviderEmptyDir(t *testing.T) {
	p := NewGitReviewProvider()
	ctx := context.Background()
	rc, err := p.Provide(ctx, "", "step-1")
	if err != nil {
		t.Fatalf("Provide: %v", err)
	}
	if rc != nil {
		t.Fatalf("expected nil context for empty dir, got %+v", rc)
	}
}

func TestGitReviewProviderNonGitDir(t *testing.T) {
	p := NewGitReviewProvider()
	tmpDir := t.TempDir()
	ctx := context.Background()
	rc, err := p.Provide(ctx, tmpDir, "step-1")
	if err != nil {
		t.Fatalf("Provide: %v", err)
	}
	if rc == nil {
		t.Fatal("expected non-nil context")
	}
	if !rc.Stale {
		t.Fatal("expected stale=true for non-git dir")
	}
}

func TestGitReviewProviderInGitRepo(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	p := NewGitReviewProvider()
	ctx := context.Background()

	rc, err := p.Provide(ctx, repoDir, "step-1")
	if err != nil {
		t.Fatalf("Provide: %v", err)
	}
	if rc == nil {
		t.Fatal("expected non-nil context")
	}
	if rc.Provider != "git" {
		t.Fatalf("provider = %q, want git", rc.Provider)
	}
	if rc.Risk == "" {
		t.Fatal("expected non-empty risk")
	}
}

func TestAssessRisk(t *testing.T) {
	tests := []struct {
		files []string
		want  string
	}{
		{nil, "low"},
		{[]string{}, "low"},
		{[]string{"README.md"}, "low"},
		{[]string{"internal/engine/engine.go"}, "medium"},
		{[]string{"go.mod"}, "high"},
		{[]string{"go.mod", "internal/engine/engine.go"}, "high"},
		{[]string{"client/Sources/View.swift"}, "medium"},
	}
	for _, tc := range tests {
		got := assessRisk(tc.files)
		if got != tc.want {
			t.Errorf("assessRisk(%v) = %q, want %q", tc.files, got, tc.want)
		}
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		path, pattern string
		want          bool
	}{
		{"foo_test.go", "*_test.go", true},
		{"bar_test.go", "*_test.go", true},
		{"foo.go", "*_test.go", false},
		{"docs/readme.md", "docs/*", true},
		{"docs/sub/readme.md", "docs/*", true},
		{"anything", "*", true},
	}
	for _, tc := range tests {
		got := matchGlob(tc.path, tc.pattern)
		if got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.path, tc.pattern, got, tc.want)
		}
	}
}

func TestUnique(t *testing.T) {
	got := unique([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("unique = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unique[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("a\nb\nc\n")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("splitLines = %v, want [a b c]", got)
	}
	got = splitLines("")
	if len(got) != 0 {
		t.Fatalf("splitLines empty = %v, want []", got)
	}
	got = splitLines("single")
	if len(got) != 1 || got[0] != "single" {
		t.Fatalf("splitLines single = %v, want [single]", got)
	}
}

func TestReviewContextSchemaVersion(t *testing.T) {
	if ReviewContextSchemaVersion != 1 {
		t.Fatalf("ReviewContextSchemaVersion = %d, want 1", ReviewContextSchemaVersion)
	}
}

func TestEngineHasReviewProvider(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	eng := New(cfg, config.NewResolver(cfg), &countingRunner{}, NewBus())
	if eng.reviewProvider == nil {
		t.Fatal("expected non-nil review provider")
	}
	if eng.reviewProvider.Name() != "git" {
		t.Fatalf("review provider name = %q, want git", eng.reviewProvider.Name())
	}
}

func TestEngineSetsReviewProvider(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfg := config.Default()
	eng := New(cfg, config.NewResolver(cfg), &countingRunner{}, NewBus())
	// Verify that the provider is reachable from the engine.
	if eng.reviewProvider == nil {
		t.Fatal("reviewProvider should not be nil")
	}
}

func BenchmarkMatchGlob(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matchGlob("internal/engine/engine.go", "*_test.go")
	}
}

// Test that ReviewContext serializes to JSON without errors.
func TestReviewContextJSON(t *testing.T) {
	rc := ReviewContext{
		SchemaVersion: 1,
		Provider:      "git",
		GeneratedAt:   1234567890,
		ChangedFiles:  []string{"file1.go", "file2.go"},
		Risk:          "medium",
	}
	data, err := json.Marshal(rc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "changed_files") {
		t.Fatalf("JSON missing changed_files: %s", data)
	}
}
