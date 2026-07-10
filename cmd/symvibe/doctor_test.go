package main

import (
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

func TestConfiguredModelIDs(t *testing.T) {
	cfg := config.Default()
	cfg.Models = map[string]config.Model{
		"gpt4o":  {ID: "openai/gpt-4o", FallbackModels: []string{"anthropic/claude-3-opus"}},
		"sonnet": {ID: "anthropic/claude-3-5-sonnet"},
	}

	ids := configuredModelIDs(cfg)
	if len(ids) != 3 {
		t.Fatalf("expected 3 unique ids (2 base + 1 fallback), got %d: %v", len(ids), ids)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate id %q in result", id)
		}
		seen[id] = true
	}
	if !seen["openai/gpt-4o"] || !seen["anthropic/claude-3-opus"] || !seen["anthropic/claude-3-5-sonnet"] {
		t.Fatalf("missing expected model id: %v", ids)
	}
}

func TestStatus(t *testing.T) {
	if got := status(false, "", "", "missing"); got != "✕ missing" {
		t.Fatalf("status(false) = %q, want ✕ missing", got)
	}
	if got := status(true, "1.2.3", "/bin/x", ""); got != "✓ v1.2.3  (/bin/x)" {
		t.Fatalf("status(true) = %q, want ✓ v1.2.3  (/bin/x)", got)
	}
	if got := status(true, "", "", "ok"); got != "✓" {
		t.Fatalf("status(true, empty) = %q, want ✓", got)
	}
}

func TestOnPath(t *testing.T) {
	// PATH always contains something executable on a dev machine; test a known
	// binary first, then an impossible one.
	if !onPath("go") {
		t.Fatal("expected 'go' to be on PATH")
	}
	if onPath("symvibe-definitely-not-on-path-xyz") {
		t.Fatal("unexpected nonexistent binary found on PATH")
	}
}
