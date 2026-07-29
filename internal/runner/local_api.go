package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/pricing"
)

type LocalAPIRunner struct {
	endpoint string
	token    string
	model    string
	http     *http.Client
	timeout  time.Duration
}

func NewLocalAPIRunner(endpoint, token, model string, timeout time.Duration) *LocalAPIRunner {
	endpoint = strings.TrimRight(endpoint, "/")
	return &LocalAPIRunner{
		endpoint: endpoint,
		token:    token,
		model:    model,
		http:     &http.Client{Timeout: 0},
		timeout:  timeout,
	}
}

func (r *LocalAPIRunner) Name() string { return "local_api" }

func (r *LocalAPIRunner) Available(ctx context.Context) (bool, Info) {
	if r.endpoint == "" {
		return false, Info{Name: "local_api", Detail: "no local_api_endpoint configured"}
	}
	return true, Info{Name: "local_api", Version: r.model, Detail: r.endpoint}
}

func (r *LocalAPIRunner) RunStep(ctx context.Context, req StepRequest) (<-chan RunEvent, error) {
	if r.endpoint == "" {
		return nil, ErrUnavailable
	}

	runCtx := ctx
	cancel := context.CancelFunc(func() {})
	if r.timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, r.timeout)
	}

	model := r.model
	if req.Model != "" {
		model = req.Model
	}

	body, err := openAIRequestBody(model, req.Message)
	if err != nil {
		cancel()
		return nil, err
	}

	apiURL := r.endpoint + "/chat/completions"
	hreq, err := http.NewRequestWithContext(runCtx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	hreq.Header.Set("content-type", "application/json")
	hreq.Header.Set("accept", "text/event-stream")
	if r.token != "" {
		hreq.Header.Set("authorization", "Bearer "+r.token)
	}

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
			msg := localAPIErrorMessage(resp)
			emit(ch, runCtx, RunEvent{Kind: EventStart, Text: "local API request started"})
			emit(ch, runCtx, RunEvent{Kind: EventError, Err: msg, Text: msg})
			emit(ch, runCtx, RunEvent{Kind: EventDone, Text: "failed", Err: msg})
			return
		}

		emit(ch, runCtx, RunEvent{Kind: EventStart, Text: "local API request started"})

		var firstErr string
		var totalInput, totalOutput int
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 8<<20)
		for sc.Scan() {
			ev, delta := parseOpenAISSELine(sc.Bytes())
			if ev == nil {
				continue
			}
			if ev.Usage != nil {
				if ev.Usage.InputTokens > totalInput {
					totalInput = ev.Usage.InputTokens
				}
				if ev.Usage.OutputTokens > totalOutput {
					totalOutput = ev.Usage.OutputTokens
				}
			}
			switch ev.Kind {
			case EventError:
				if firstErr == "" {
					firstErr = firstNonEmpty(ev.Err, ev.Text)
				}
				emit(ch, runCtx, *ev)
			case EventLog:
				if delta {
					emit(ch, runCtx, *ev)
				}
			}
		}
		if err := sc.Err(); err != nil && firstErr == "" {
			firstErr = err.Error()
		}

		done := RunEvent{Kind: EventDone}
		if totalInput > 0 || totalOutput > 0 {
			u := &Usage{
				InputTokens:  totalInput,
				OutputTokens: totalOutput,
				Model:        req.Model,
			}
			if cost, ok := pricing.CalculateCost(req.Model, totalInput, totalOutput, 0); ok {
				u.CostUSD = cost
			}
			done.Usage = u
		}
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

type openAIReq struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func openAIRequestBody(model, userMsg string) ([]byte, error) {
	req := openAIReq{
		Model:    model,
		Messages: []openAIMessage{{Role: "user", Content: userMsg}},
		Stream:   true,
	}
	return json.Marshal(req)
}

type openAISSE struct {
	Choices []openAIChoice `json:"choices"`
	Error   *openAIError   `json:"error"`
	Usage   *openAIUsage   `json:"usage"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openAIChoice struct {
	Delta openAIDelta `json:"delta"`
}

type openAIDelta struct {
	Content string `json:"content"`
}

type openAIError struct {
	Message string `json:"message"`
}

func parseOpenAISSELine(line []byte) (*RunEvent, bool) {
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

	var e openAISSE
	if err := json.Unmarshal(data, &e); err != nil {
		return &RunEvent{Kind: EventLog, Text: string(data), Raw: string(data)}, false
	}

	var u *Usage
	if e.Usage != nil && (e.Usage.PromptTokens > 0 || e.Usage.CompletionTokens > 0) {
		u = &Usage{
			InputTokens:  e.Usage.PromptTokens,
			OutputTokens: e.Usage.CompletionTokens,
		}
	}

	if e.Error != nil && e.Error.Message != "" {
		text := "local API error: " + e.Error.Message
		return &RunEvent{Kind: EventError, Err: text, Text: text, Usage: u, Raw: string(data)}, false
	}

	if len(e.Choices) > 0 && e.Choices[0].Delta.Content != "" {
		return &RunEvent{Kind: EventLog, Text: e.Choices[0].Delta.Content, Usage: u, Raw: string(data)}, true
	}

	if u != nil {
		return &RunEvent{Kind: EventLog, Usage: u, Raw: string(data)}, false
	}

	return nil, false
}

func localAPIErrorMessage(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Sprintf("local API %s: %s", resp.Status, errResp.Error.Message)
	}
	return fmt.Sprintf("local API %s", resp.Status)
}
