package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var anthropicAPIURL = "https://api.anthropic.com/v1/messages"

const anthropicAPIVersion = "2023-06-01"

// APIRunner drives the Anthropic Messages API directly. It lets symvibe run
// steps without a local opencode installation, turning the heaviest external
// dependency into an optional power-user backend.
type APIRunner struct {
	apiKey  string
	http    *http.Client
	timeout time.Duration
}

// NewAPIRunner creates an Anthropic API backend. An empty apiKey produces a
// runner that reports unavailable; this keeps the engine construction path
// simple and failures localized to Available/RunStep.
func NewAPIRunner(apiKey string, timeout time.Duration) *APIRunner {
	return &APIRunner{
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: 0},
		timeout: timeout,
	}
}

func (r *APIRunner) Name() string { return "api" }

// Available reports whether an Anthropic API key is configured. We do not make
// a live API call here so doctor stays fast and offline-friendly; RunStep will
// surface auth/network errors as terminal events.
func (r *APIRunner) Available(ctx context.Context) (bool, Info) {
	if r.apiKey == "" {
		if env := os.Getenv("SYMVIBE_ANTHROPIC_API_KEY"); strings.TrimSpace(env) != "" {
			return true, Info{Name: "api", Version: "anthropic", Detail: "API key from environment"}
		}
		return false, Info{Name: "api", Version: "anthropic", Detail: "no API key: set runner.api_key or SYMVIBE_ANTHROPIC_API_KEY"}
	}
	masked := "***"
	if len(r.apiKey) > 7 {
		masked = r.apiKey[:4] + "..." + r.apiKey[len(r.apiKey)-4:]
	}
	return true, Info{Name: "api", Version: "anthropic", Detail: "API key " + masked}
}

// RunStep calls the Anthropic Messages API and streams normalized events. The
// composed Message is sent as a single user turn; the assistant response is
// emitted as log events. Tool use blocks are surfaced as tool events.
func (r *APIRunner) RunStep(ctx context.Context, req StepRequest) (<-chan RunEvent, error) {
	apiKey := r.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("SYMVIBE_ANTHROPIC_API_KEY")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, ErrUnavailable
	}

	runCtx := ctx
	cancel := context.CancelFunc(func() {})
	if r.timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, r.timeout)
	}

	model := anthropicModelID(req.Model, req.Variant)
	body, err := anthropicRequestBody(model, req.Message)
	if err != nil {
		cancel()
		return nil, err
	}

	hreq, err := http.NewRequestWithContext(runCtx, http.MethodPost, anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	hreq.Header.Set("x-api-key", apiKey)
	hreq.Header.Set("anthropic-version", anthropicAPIVersion)
	hreq.Header.Set("content-type", "application/json")
	hreq.Header.Set("accept", "text/event-stream")

	resp, err := r.http.Do(hreq)
	if err != nil {
		cancel()
		return nil, err
	}

	ch := make(chan RunEvent, 128)
	go func() {
		defer close(ch)
		defer cancel()
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			msg := anthropicErrorMessage(resp)
			emit(ch, runCtx, RunEvent{Kind: EventStart, Text: "Anthropic API request started"})
			emit(ch, runCtx, RunEvent{Kind: EventError, Err: msg, Text: msg})
			emit(ch, runCtx, RunEvent{Kind: EventDone, Text: "failed", Err: msg})
			return
		}

		emit(ch, runCtx, RunEvent{Kind: EventStart, Text: "Anthropic API request started"})

		var firstErr string
		var textBuf strings.Builder
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 8<<20)
		for sc.Scan() {
			ev, delta := parseSSELine(sc.Bytes())
			if ev == nil {
				continue
			}
			switch ev.Kind {
			case EventError:
				if firstErr == "" {
					firstErr = firstNonEmpty(ev.Err, ev.Text)
				}
				emit(ch, runCtx, *ev)
			case EventTool:
				emit(ch, runCtx, *ev)
			case EventLog:
				if delta {
					textBuf.WriteString(ev.Text)
				}
				emit(ch, runCtx, *ev)
			}
		}
		if err := sc.Err(); err != nil && firstErr == "" {
			firstErr = err.Error()
		}

		done := RunEvent{Kind: EventDone}
		switch {
		case runCtx.Err() == context.DeadlineExceeded:
			done.Err = "step timed out"
		case ctx.Err() == context.Canceled:
			done.Err = "cancelled"
		case firstErr != "":
			done.Err = firstErr
		default:
			done.Text = "completed"
		}
		if done.Err != "" {
			done.Text = "failed"
		}
		emit(ch, runCtx, done)
	}()

	return ch, nil
}

type messagesReq struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
	Stream    bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func anthropicRequestBody(model, userMsg string) ([]byte, error) {
	req := messagesReq{
		Model:     model,
		MaxTokens: 8192,
		Messages:  []message{{Role: "user", Content: userMsg}},
		Stream:    true,
	}
	return json.Marshal(req)
}

// parseSSELine turns one Anthropic streaming SSE line into a RunEvent.
// Returns (event, isTextDelta) so the caller can optionally keep a buffer.
func parseSSELine(line []byte) (*RunEvent, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil, false
	}
	if !bytes.HasPrefix(line, []byte("data: ")) {
		return nil, false
	}
	data := bytes.TrimSpace(line[6:])
	if bytes.Equal(data, []byte("[DONE]")) {
		return nil, false
	}

	var e sseEvent
	if err := json.Unmarshal(data, &e); err != nil {
		return &RunEvent{Kind: EventLog, Text: string(data), Raw: string(data)}, false
	}

	switch e.Type {
	case "content_block_delta":
		if e.Delta.Type == "text_delta" {
			return &RunEvent{Kind: EventLog, Text: e.Delta.Text, Raw: string(data)}, true
		}
		if e.Delta.Type == "input_json_delta" {
			return &RunEvent{Kind: EventTool, Text: e.Delta.PartialJSON, Raw: string(data)}, false
		}
	case "content_block_start":
		if e.ContentBlock.Type == "tool_use" {
			return &RunEvent{Kind: EventTool, Text: e.ContentBlock.Name, Raw: string(data)}, false
		}
		if e.ContentBlock.Type == "text" && e.ContentBlock.Text != "" {
			return &RunEvent{Kind: EventLog, Text: e.ContentBlock.Text, Raw: string(data)}, true
		}
	case "message_delta":
		if e.Delta.StopReason == "error" || e.Delta.StopReason == "max_tokens" {
			text := "stopped: " + e.Delta.StopReason
			return &RunEvent{Kind: EventError, Err: text, Text: text, Raw: string(data)}, false
		}
	case "error":
		text := firstNonEmpty(e.Error.Message, "Anthropic API error")
		return &RunEvent{Kind: EventError, Err: text, Text: text, Raw: string(data)}, false
	}
	return nil, false
}

type sseEvent struct {
	Type         string       `json:"type"`
	Delta        sseDelta     `json:"delta"`
	ContentBlock contentBlock `json:"content_block"`
	Error        apiError     `json:"error"`
}

type sseDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
	StopReason  string `json:"stop_reason"`
}

type contentBlock struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Text string `json:"text"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func anthropicModelID(requested, variant string) string {
	if strings.Contains(requested, "claude") {
		_, model := SplitModel(requested)
		if model != "" {
			return model
		}
		return requested
	}
	switch variant {
	case "max", "high":
		return "claude-sonnet-4-20250514"
	case "minimal":
		return "claude-haiku-4-20250514"
	default:
		return "claude-sonnet-4-20250514"
	}
}

func anthropicErrorMessage(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Sprintf("Anthropic API %s: %s", resp.Status, errResp.Error.Message)
	}
	return fmt.Sprintf("Anthropic API %s", resp.Status)
}
