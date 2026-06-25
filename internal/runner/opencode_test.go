package runner

import (
	"context"
	"testing"
	"time"
)

func TestOpenCodeRunnerName(t *testing.T) {
	r := &OpenCodeRunner{bin: "/usr/bin/opencode"}
	if r.Name() != "opencode" {
		t.Fatalf("want 'opencode', got %q", r.Name())
	}
}

func TestOpenCodeRunnerAvailableNoBinary(t *testing.T) {
	r := &OpenCodeRunner{bin: ""}
	ok, info := r.Available(context.Background())
	if ok {
		t.Fatal("expected unavailable when bin is empty")
	}
	if info.Name != "opencode" {
		t.Fatalf("want name 'opencode', got %q", info.Name)
	}
	if info.Detail != "not found on PATH or ~/.opencode/bin" {
		t.Fatalf("want detail 'not found on PATH or ~/.opencode/bin', got %q", info.Detail)
	}
}

func TestOpenCodeRunnerAvailableInvalidBinary(t *testing.T) {
	r := &OpenCodeRunner{bin: "/nonexistent/binary"}
	ok, info := r.Available(context.Background())
	if ok {
		t.Fatal("expected unavailable for nonexistent binary")
	}
	if info.Name != "opencode" {
		t.Fatalf("want name 'opencode', got %q", info.Name)
	}
}

func TestOpenCodeRunnerRunStepUnavailable(t *testing.T) {
	r := &OpenCodeRunner{bin: ""}
	ch, err := r.RunStep(context.Background(), StepRequest{Message: "hello"})
	if err != ErrUnavailable {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
	if ch != nil {
		t.Fatal("expected nil channel")
	}
}

func TestBuildArgsMinimal(t *testing.T) {
	req := StepRequest{Message: "hello"}
	args := buildArgs(req)

	if len(args) < 3 {
		t.Fatalf("expected at least 3 args, got %d", len(args))
	}
	if args[0] != "run" {
		t.Fatalf("want first arg 'run', got %q", args[0])
	}
	if args[1] != "--format" {
		t.Fatalf("want second arg '--format', got %q", args[1])
	}
	if args[2] != "json" {
		t.Fatalf("want third arg 'json', got %q", args[2])
	}
	if args[len(args)-1] != "hello" {
		t.Fatalf("want last arg 'hello', got %q", args[len(args)-1])
	}
}

func TestBuildArgsWithAgent(t *testing.T) {
	req := StepRequest{Message: "hello", Agent: "build"}
	args := buildArgs(req)

	found := false
	for i, arg := range args {
		if arg == "--agent" && i+1 < len(args) && args[i+1] == "build" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected --agent build in args")
	}
}

func TestBuildArgsWithModel(t *testing.T) {
	req := StepRequest{Message: "hello", Model: "claude-sonnet-4-20250514"}
	args := buildArgs(req)

	found := false
	for i, arg := range args {
		if arg == "--model" && i+1 < len(args) && args[i+1] == "claude-sonnet-4-20250514" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected --model claude-sonnet-4-20250514 in args")
	}
}

func TestBuildArgsWithVariant(t *testing.T) {
	req := StepRequest{Message: "hello", Variant: "fast"}
	args := buildArgs(req)

	found := false
	for i, arg := range args {
		if arg == "--variant" && i+1 < len(args) && args[i+1] == "fast" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected --variant fast in args")
	}
}

func TestBuildArgsWithWorkingDir(t *testing.T) {
	req := StepRequest{Message: "hello", WorkingDir: "/tmp/test"}
	args := buildArgs(req)

	found := false
	for i, arg := range args {
		if arg == "--dir" && i+1 < len(args) && args[i+1] == "/tmp/test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected --dir /tmp/test in args")
	}
}

func TestBuildArgsWithSkipPerms(t *testing.T) {
	req := StepRequest{Message: "hello", SkipPerms: true}
	args := buildArgs(req)

	found := false
	for _, arg := range args {
		if arg == "--dangerously-skip-permissions" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected --dangerously-skip-permissions in args")
	}
}

func TestBuildArgsEmptyMessage(t *testing.T) {
	req := StepRequest{Message: "   "}
	args := buildArgs(req)

	// Empty message should not be included
	for _, arg := range args {
		if arg == "   " {
			t.Fatal("empty message should not be included in args")
		}
	}
}

func TestNewOpenCodeRunner(t *testing.T) {
	r := NewOpenCodeRunner("/usr/bin/opencode", 30*time.Second)
	if r.timeout != 30*time.Second {
		t.Fatalf("want timeout 30s, got %v", r.timeout)
	}
}

func TestNewOpenCodeRunnerDefaultTimeout(t *testing.T) {
	r := NewOpenCodeRunner("/usr/bin/opencode", 0)
	if r.timeout != 0 {
		t.Fatalf("want timeout 0, got %v", r.timeout)
	}
}
