package runner

import (
	"errors"
	"strings"
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

func TestNewOpencodeDefault(t *testing.T) {
	r, err := New(config.RunnerConfig{Backend: ""})
	if err != nil {
		t.Fatalf("unexpected error for empty backend: %v", err)
	}
	if r.Name() != "opencode" {
		t.Fatalf("want 'opencode', got %q", r.Name())
	}
}

func TestNewOpencodeExplicit(t *testing.T) {
	r, err := New(config.RunnerConfig{Backend: "opencode"})
	if err != nil {
		t.Fatalf("unexpected error for opencode backend: %v", err)
	}
	if r.Name() != "opencode" {
		t.Fatalf("want 'opencode', got %q", r.Name())
	}
}

func TestNewAPI(t *testing.T) {
	r, err := New(config.RunnerConfig{Backend: "api", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("unexpected error for api backend: %v", err)
	}
	if r.Name() != "api" {
		t.Fatalf("want 'api', got %q", r.Name())
	}
}

func TestNewClaudecodeNotImplemented(t *testing.T) {
	_, err := New(config.RunnerConfig{Backend: "claudecode"})
	if err == nil {
		t.Fatal("expected error for claudecode backend")
	}
	if !errors.Is(err, ErrUnsupportedBackend) {
		t.Fatalf("want ErrUnsupportedBackend, got %v", err)
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("want 'not yet implemented' in error, got %q", err.Error())
	}
}

func TestNewUnknownBackend(t *testing.T) {
	_, err := New(config.RunnerConfig{Backend: "typo"})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
	if !errors.Is(err, ErrUnsupportedBackend) {
		t.Fatalf("want ErrUnsupportedBackend, got %v", err)
	}
	if !strings.Contains(err.Error(), "typo") {
		t.Fatalf("want 'typo' in error message, got %q", err.Error())
	}
}

func TestNewAnotherUnknown(t *testing.T) {
	_, err := New(config.RunnerConfig{Backend: "ollama"})
	if err == nil {
		t.Fatal("expected error for ollama backend")
	}
	if !errors.Is(err, ErrUnsupportedBackend) {
		t.Fatalf("want ErrUnsupportedBackend, got %v", err)
	}
}
