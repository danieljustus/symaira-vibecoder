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

func TestNewClaudecodeBinaryNotFound(t *testing.T) {
	// Use a binary name that doesn't exist on PATH to ensure the factory returns an error.
	// The configured path takes precedence, and if it doesn't exist AND the name isn't on PATH, we get an error.
	_, err := New(config.RunnerConfig{
		Backend:      "claudecode",
		ClaudeCodeBin: "/nonexistent/path/to/nonexistent-binary-xyz",
	})
	if err == nil {
		t.Fatal("expected error for claudecode backend when binary not found")
	}
	if !errors.Is(err, ErrUnsupportedBackend) {
		t.Fatalf("want ErrUnsupportedBackend, got %v", err)
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

func TestNewLocalAPI(t *testing.T) {
	r, err := New(config.RunnerConfig{
		Backend:          "local_api",
		LocalAPIEndpoint: "http://localhost:11434/v1",
		LocalAPIModel:    "llama3",
	})
	if err != nil {
		t.Fatalf("unexpected error for local_api backend: %v", err)
	}
	if r.Name() != "local_api" {
		t.Fatalf("want 'local_api', got %q", r.Name())
	}
}

func TestNewAiderBinaryNotFound(t *testing.T) {
	_, err := New(config.RunnerConfig{
		Backend:  "aider",
		AiderBin: "/nonexistent/path/to/aider",
	})
	if err == nil {
		t.Fatal("expected error for aider backend when binary not found")
	}
	if !errors.Is(err, ErrUnsupportedBackend) {
		t.Fatalf("want ErrUnsupportedBackend, got %v", err)
	}
}

func TestNewClineBinaryNotFound(t *testing.T) {
	_, err := New(config.RunnerConfig{
		Backend:  "cline",
		ClineBin: "/nonexistent/path/to/cline",
	})
	if err == nil {
		t.Fatal("expected error for cline backend when binary not found")
	}
	if !errors.Is(err, ErrUnsupportedBackend) {
		t.Fatalf("want ErrUnsupportedBackend, got %v", err)
	}
}
