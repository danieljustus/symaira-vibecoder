package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAPIRunnerAvailable(t *testing.T) {
	r := NewAPIRunner("sk-test-123", 0)
	ok, info := r.Available(context.Background())
	if !ok {
		t.Fatalf("expected available with API key")
	}
	if info.Name != "api" {
		t.Fatalf("expected name api, got %q", info.Name)
	}
	if !strings.Contains(info.Detail, "sk-") || !strings.Contains(info.Detail, "...") {
		t.Fatalf("expected masked key in detail, got %q", info.Detail)
	}

	r2 := NewAPIRunner("", 0)
	ok2, info2 := r2.Available(context.Background())
	if ok2 {
		t.Fatalf("expected unavailable without API key")
	}
	if !strings.Contains(info2.Detail, "no API key") {
		t.Fatalf("expected missing-key hint, got %q", info2.Detail)
	}
}

func TestAPIRunnerRunStepUnavailable(t *testing.T) {
	r := NewAPIRunner("", 0)
	ch, err := r.RunStep(context.Background(), StepRequest{Message: "hello"})
	if err != ErrUnavailable {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
	if ch != nil {
		t.Fatalf("expected nil channel")
	}
}

func TestAPIRunnerRunStepSuccess(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		lines := []string{
			"data: {\"type\":\"message_start\",\"message\":{\"role\":\"assistant\"}}",
			"data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"text\",\"text\":\"Hello\"}}",
			"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}",
			"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}",
			"data: [DONE]",
		}
		for _, line := range lines {
			fmt.Fprintln(w, line)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	// Point the runner at the test server.
	r := NewAPIRunner("sk-test", time.Minute)
	r.http = srv.Client()
	anthropicAPIURL = srv.URL
	defer func() { anthropicAPIURL = "https://api.anthropic.com/v1/messages" }()

	ch, err := r.RunStep(context.Background(), StepRequest{Message: "say hi", Model: "claude-sonnet-4-20250514"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var kinds []RunEventKind
	var texts []string
	for ev := range ch {
		kinds = append(kinds, ev.Kind)
		if ev.Text != "" {
			texts = append(texts, ev.Text)
		}
	}

	if gotHeaders.Get("x-api-key") != "sk-test" {
		t.Fatalf("expected API key header, got %q", gotHeaders.Get("x-api-key"))
	}
	if gotHeaders.Get("anthropic-version") != anthropicAPIVersion {
		t.Fatalf("expected version header, got %q", gotHeaders.Get("anthropic-version"))
	}

	wantStart := EventStart
	wantLog := EventLog
	wantDone := EventDone
	if len(kinds) < 3 || kinds[0] != wantStart || kinds[len(kinds)-1] != wantDone {
		t.Fatalf("expected start ... done, got %v", kinds)
	}
	if kinds[1] != wantLog {
		t.Fatalf("expected second event to be log, got %v", kinds[1])
	}
	joined := strings.Join(texts, "")
	if !strings.Contains(joined, "Hello world") {
		t.Fatalf("expected response text, got %q", joined)
	}
}

func TestAPIRunnerRunStepHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"type": "authentication_error", "message": "invalid key"},
		})
	}))
	defer srv.Close()

	r := NewAPIRunner("sk-bad", time.Minute)
	r.http = srv.Client()
	anthropicAPIURL = srv.URL
	defer func() { anthropicAPIURL = "https://api.anthropic.com/v1/messages" }()

	ch, _ := r.RunStep(context.Background(), StepRequest{Message: "x"})
	var sawDone, sawError bool
	for ev := range ch {
		if ev.Kind == EventError {
			sawError = true
		}
		if ev.Kind == EventDone {
			sawDone = true
			if ev.Err == "" {
				t.Fatalf("expected terminal error")
			}
			if !strings.Contains(ev.Err, "invalid key") {
				t.Fatalf("expected error message, got %q", ev.Err)
			}
		}
	}
	if !sawError || !sawDone {
		t.Fatalf("expected error and done events")
	}
}

func TestAPIRunnerRunStepCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 1000; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"x\"}}\n")
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer srv.Close()

	r := NewAPIRunner("sk-test", time.Minute)
	r.http = srv.Client()
	anthropicAPIURL = srv.URL
	defer func() { anthropicAPIURL = "https://api.anthropic.com/v1/messages" }()

	ctx, cancel := context.WithCancel(context.Background())
	ch, _ := r.RunStep(ctx, StepRequest{Message: "x"})
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	var sawDone bool
	for ev := range ch {
		if ev.Kind == EventDone {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatalf("expected done after cancellation")
	}
}

func TestAnthropicModelID(t *testing.T) {
	cases := []struct {
		req, variant, want string
	}{
		{"anthropic/claude-sonnet-4-20250514", "", "claude-sonnet-4-20250514"},
		{"claude-opus-4-20250514", "", "claude-opus-4-20250514"},
		{"opencode-go/mimo-v2.5", "max", "claude-sonnet-4-20250514"},
		{"opencode-go/mimo-v2.5", "high", "claude-sonnet-4-20250514"},
		{"opencode-go/mimo-v2.5", "minimal", "claude-haiku-4-20250514"},
		{"opencode-go/mimo-v2.5", "", "claude-sonnet-4-20250514"},
	}
	for _, c := range cases {
		got := anthropicModelID(c.req, c.variant)
		if got != c.want {
			t.Errorf("anthropicModelID(%q, %q) = %q, want %q", c.req, c.variant, got, c.want)
		}
	}
}

func TestParseSSELine(t *testing.T) {
	cases := []struct {
		line      string
		wantKind  RunEventKind
		wantText  string
		wantDelta bool
	}{
		{"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}", EventLog, "hi", true},
		{"data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"tool_use\",\"name\":\"git\"}}", EventTool, "git", false},
		{"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"max_tokens\"}}", EventError, "stopped: max_tokens", false},
		{"data: {\"type\":\"error\",\"error\":{\"type\":\"rate_limit\",\"message\":\"slow down\"}}", EventError, "slow down", false},
		{"event: ping\n\n", "", "", false},
		{"data: [DONE]", "", "", false},
	}
	for _, c := range cases {
		ev, delta := parseSSELine([]byte(c.line))
		if c.wantKind == "" {
			if ev != nil {
				t.Errorf("%q: expected nil event, got %+v", c.line, ev)
			}
			continue
		}
		if ev == nil {
			t.Errorf("%q: expected event", c.line)
			continue
		}
		if ev.Kind != c.wantKind || ev.Text != c.wantText || delta != c.wantDelta {
			t.Errorf("%q: got kind=%q text=%q delta=%v, want kind=%q text=%q delta=%v",
				c.line, ev.Kind, ev.Text, delta, c.wantKind, c.wantText, c.wantDelta)
		}
	}
}

func TestAnthropicRequestBody(t *testing.T) {
	body, err := anthropicRequestBody("claude-sonnet-4-20250514", "hello")
	if err != nil {
		t.Fatal(err)
	}
	var req messagesReq
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if req.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("model mismatch: %q", req.Model)
	}
	if !req.Stream {
		t.Fatalf("expected streaming")
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "hello" {
		t.Fatalf("unexpected messages: %+v", req.Messages)
	}
}

func TestAnthropicErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "plan expired"},
		})
	}))
	defer srv.Close()

	resp, _ := http.Get(srv.URL)
	msg := anthropicErrorMessage(resp)
	resp.Body.Close()
	if !strings.Contains(msg, "Forbidden") || !strings.Contains(msg, "plan expired") {
		t.Fatalf("unexpected error message: %q", msg)
	}
}

func TestAPIRunnerRequestBodyReadable(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
	}))
	defer srv.Close()

	r := NewAPIRunner("sk-test", time.Minute)
	r.http = srv.Client()
	anthropicAPIURL = srv.URL
	defer func() { anthropicAPIURL = "https://api.anthropic.com/v1/messages" }()

	ch, _ := r.RunStep(context.Background(), StepRequest{Message: "do it", Model: "opencode-go/mimo-v2.5"})
	for range ch {
	}

	var req messagesReq
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if req.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("expected fallback model, got %q", req.Model)
	}
}
