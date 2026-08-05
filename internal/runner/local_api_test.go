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

func TestLocalAPIRunnerAvailable(t *testing.T) {
	r := NewLocalAPIRunner("http://127.0.0.1:11434", "", "llama3.1:8b", 0)
	ok, info := r.Available(context.Background())
	if !ok {
		t.Fatalf("expected available with endpoint")
	}
	if info.Name != "local_api" {
		t.Fatalf("expected name local_api, got %q", info.Name)
	}
	if info.Version != "llama3.1:8b" {
		t.Fatalf("expected model as version, got %q", info.Version)
	}
	if info.Detail != "http://127.0.0.1:11434" {
		t.Fatalf("expected endpoint detail, got %q", info.Detail)
	}

	r2 := NewLocalAPIRunner("", "", "", 0)
	ok2, info2 := r2.Available(context.Background())
	if ok2 {
		t.Fatalf("expected unavailable without endpoint")
	}
	if !strings.Contains(info2.Detail, "no local_api_endpoint") {
		t.Fatalf("expected missing-endpoint hint, got %q", info2.Detail)
	}
}

func TestLocalAPIRunnerRunStepUnavailable(t *testing.T) {
	r := NewLocalAPIRunner("", "", "", 0)
	ch, err := r.RunStep(context.Background(), StepRequest{Message: "hello"})
	if err != ErrUnavailable {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
	if ch != nil {
		t.Fatalf("expected nil channel")
	}
}

func TestLocalAPIRunnerRunStepSuccess(t *testing.T) {
	var gotHeaders http.Header
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		lines := []string{
			`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"choices":[{"delta":{"content":" world"}}]}`,
			`data: {"choices":[{"delta":{"content":""}}],"usage":{"prompt_tokens":100,"completion_tokens":20}}`,
			`data: {"choices":[{"delta":{"content":""}}],"usage":{"prompt_tokens":50,"completion_tokens":40}}`,
			"data: [DONE]",
		}
		for _, line := range lines {
			_, _ = fmt.Fprintln(w, line)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	r := NewLocalAPIRunner(srv.URL, "sk-local", "llama3.1:8b", time.Minute)
	r.http = srv.Client()

	ch, err := r.RunStep(context.Background(), StepRequest{Message: "say hi", Model: "qwen2.5:7b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var kinds []RunEventKind
	var texts []string
	var done *RunEvent
	for ev := range ch {
		kinds = append(kinds, ev.Kind)
		if ev.Text != "" {
			texts = append(texts, ev.Text)
		}
		if ev.Kind == EventDone {
			done = &ev
		}
	}

	if gotHeaders.Get("Content-Type") != "application/json" {
		t.Fatalf("expected json content-type, got %q", gotHeaders.Get("Content-Type"))
	}
	if gotHeaders.Get("Accept") != "text/event-stream" {
		t.Fatalf("expected sse accept, got %q", gotHeaders.Get("Accept"))
	}
	if gotHeaders.Get("Authorization") != "Bearer sk-local" {
		t.Fatalf("expected bearer token, got %q", gotHeaders.Get("Authorization"))
	}

	var req openAIReq
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatal(err)
	}
	if req.Model != "qwen2.5:7b" {
		t.Fatalf("expected request model override, got %q", req.Model)
	}
	if !req.Stream {
		t.Fatalf("expected streaming request")
	}
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != "say hi" {
		t.Fatalf("unexpected messages: %+v", req.Messages)
	}

	if len(kinds) < 3 || kinds[0] != EventStart || kinds[len(kinds)-1] != EventDone {
		t.Fatalf("expected start ... done, got %v", kinds)
	}
	joined := strings.Join(texts, "")
	if !strings.Contains(joined, "Hello world") {
		t.Fatalf("expected response text, got %q", joined)
	}
	if done == nil || done.Err != "" || done.Text != "completed" {
		t.Fatalf("expected clean done, got %+v", done)
	}
	if done.Usage == nil {
		t.Fatalf("expected usage on done")
	}
	if done.Usage.InputTokens != 100 || done.Usage.OutputTokens != 40 {
		t.Fatalf("expected max usage 100/40, got %d/%d", done.Usage.InputTokens, done.Usage.OutputTokens)
	}
	if done.Usage.Model != "qwen2.5:7b" {
		t.Fatalf("expected usage model, got %q", done.Usage.Model)
	}
}

func TestLocalAPIRunnerRunStepHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "invalid token"},
		})
	}))
	defer srv.Close()

	r := NewLocalAPIRunner(srv.URL, "sk-bad", "", time.Minute)
	r.http = srv.Client()

	ch, _ := r.RunStep(context.Background(), StepRequest{Message: "x"})
	var sawError, sawDone bool
	var doneErr string
	for ev := range ch {
		if ev.Kind == EventError {
			sawError = true
		}
		if ev.Kind == EventDone {
			sawDone = true
			doneErr = ev.Err
		}
	}
	if !sawError || !sawDone {
		t.Fatalf("expected error and done events")
	}
	if !strings.Contains(doneErr, "401 Unauthorized") || !strings.Contains(doneErr, "invalid token") {
		t.Fatalf("expected status+message in error, got %q", doneErr)
	}
}

func TestLocalAPIRunnerRunStepErrorFrame(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		lines := []string{
			`data: {"choices":[{"delta":{"content":"partial"}}]}`,
			`data: {"error":{"message":"model overloaded"}}`,
			"data: [DONE]",
		}
		for _, line := range lines {
			_, _ = fmt.Fprintln(w, line)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	r := NewLocalAPIRunner(srv.URL, "", "", time.Minute)
	r.http = srv.Client()

	ch, _ := r.RunStep(context.Background(), StepRequest{Message: "x"})
	var sawError bool
	var doneErr string
	for ev := range ch {
		if ev.Kind == EventError {
			sawError = true
			if !strings.Contains(ev.Text, "model overloaded") {
				t.Fatalf("expected error text, got %q", ev.Text)
			}
		}
		if ev.Kind == EventDone {
			doneErr = ev.Err
		}
	}
	if !sawError {
		t.Fatalf("expected error event from data frame")
	}
	if !strings.Contains(doneErr, "model overloaded") {
		t.Fatalf("expected done error, got %q", doneErr)
	}
}

func TestLocalAPIRunnerRunStepTimeout(t *testing.T) {
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
			_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n")
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer srv.Close()

	r := NewLocalAPIRunner(srv.URL, "", "", 50*time.Millisecond)
	r.http = srv.Client()

	ch, _ := r.RunStep(context.Background(), StepRequest{Message: "x"})
	var doneErr string
	for ev := range ch {
		if ev.Kind == EventDone {
			doneErr = ev.Err
		}
	}
	if !strings.Contains(doneErr, "timed out") {
		t.Fatalf("expected timeout error, got %q", doneErr)
	}
}

func TestLocalAPIRunnerRunStepCancelled(t *testing.T) {
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
			_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n")
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer srv.Close()

	r := NewLocalAPIRunner(srv.URL, "", "", time.Minute)
	r.http = srv.Client()

	ctx, cancel := context.WithCancel(context.Background())
	ch, _ := r.RunStep(ctx, StepRequest{Message: "x"})
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	var doneErr string
	for ev := range ch {
		if ev.Kind == EventDone {
			doneErr = ev.Err
		}
	}
	if !strings.Contains(doneErr, "cancelled") {
		t.Fatalf("expected cancelled error, got %q", doneErr)
	}
}

func TestLocalAPIRunnerRequestBodyReadable(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
	}))
	defer srv.Close()

	r := NewLocalAPIRunner(srv.URL, "", "fallback-model", time.Minute)
	r.http = srv.Client()

	ch, _ := r.RunStep(context.Background(), StepRequest{Message: "do it"})
	for range ch {
	}

	var req openAIReq
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatal(err)
	}
	if req.Model != "fallback-model" {
		t.Fatalf("expected fallback model, got %q", req.Model)
	}
}

func TestOpenAIRequestBody(t *testing.T) {
	cases := []struct {
		model, msg string
	}{
		{"llama3.1:8b", "hello"},
		{"", "empty model"},
	}
	for _, c := range cases {
		body, err := openAIRequestBody(c.model, c.msg)
		if err != nil {
			t.Fatalf("openAIRequestBody(%q, %q): %v", c.model, c.msg, err)
		}
		var req openAIReq
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Model != c.model {
			t.Errorf("model mismatch: got %q want %q", req.Model, c.model)
		}
		if !req.Stream {
			t.Errorf("expected stream=true for %q", c.model)
		}
		if len(req.Messages) != 1 || req.Messages[0].Role != "user" || req.Messages[0].Content != c.msg {
			t.Errorf("unexpected messages: %+v", req.Messages)
		}
	}
}

func TestParseOpenAISSELine(t *testing.T) {
	cases := []struct {
		line       string
		wantKind   RunEventKind
		wantText   string
		wantDelta  bool
		wantUsage  bool
		wantInTok  int
		wantOutTok int
	}{
		{`data: {"choices":[{"delta":{"content":"hi"}}]}`, EventLog, "hi", true, false, 0, 0},
		{`data: {"choices":[{"delta":{"content":"x"}}],"usage":{"prompt_tokens":3,"completion_tokens":7}}`, EventLog, "x", true, true, 3, 7},
		{`data: {"choices":[{"delta":{"content":""}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`, EventLog, "", false, true, 10, 5},
		{`data: {"error":{"message":"boom"}}`, EventError, "local API error: boom", false, false, 0, 0},
		{"data: [DONE]", "", "", false, false, 0, 0},
		{"event: ping", "", "", false, false, 0, 0},
		{"", "", "", false, false, 0, 0},
		{"   ", "", "", false, false, 0, 0},
		{"data: not-json", EventLog, "not-json", false, false, 0, 0},
		{"data: {}", "", "", false, false, 0, 0},
		{`data: {"usage":{"prompt_tokens":0,"completion_tokens":0}}`, "", "", false, false, 0, 0},
	}
	for _, c := range cases {
		ev, delta := parseOpenAISSELine([]byte(c.line))
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
		if c.wantUsage {
			if ev.Usage == nil {
				t.Errorf("%q: expected usage", c.line)
				continue
			}
			if ev.Usage.InputTokens != c.wantInTok || ev.Usage.OutputTokens != c.wantOutTok {
				t.Errorf("%q: got usage %d/%d, want %d/%d", c.line, ev.Usage.InputTokens, ev.Usage.OutputTokens, c.wantInTok, c.wantOutTok)
			}
		} else if ev.Usage != nil {
			t.Errorf("%q: unexpected usage %+v", c.line, ev.Usage)
		}
	}
}

func TestLocalAPIErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "bad token"},
		})
	}))
	defer srv.Close()

	resp, _ := http.Get(srv.URL)
	msg := localAPIErrorMessage(resp)
	_ = resp.Body.Close()
	if !strings.Contains(msg, "401 Unauthorized") || !strings.Contains(msg, "bad token") {
		t.Fatalf("unexpected error message: %q", msg)
	}

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream exploded"))
	}))
	defer srv2.Close()

	resp2, _ := http.Get(srv2.URL)
	msg2 := localAPIErrorMessage(resp2)
	_ = resp2.Body.Close()
	if msg2 != "local API 500 Internal Server Error" {
		t.Fatalf("unexpected error message: %q", msg2)
	}
}
