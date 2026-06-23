package runner

import (
	"bytes"
	"encoding/json"
)

// ocEvent is the minimal shape of one `opencode run --format json` line.
//
// Verified envelope (opencode 1.17.9): each line is a JSON object with a
// top-level "type", a millisecond "timestamp" and a "sessionID". The "error"
// type carries error.data.message. Other fields below are best-effort across
// opencode event variants; unknown types degrade to a log line carrying the raw
// payload, so the activity feed keeps working even if opencode adds event types.
type ocEvent struct {
	Type      string   `json:"type"`
	SessionID string   `json:"sessionID"`
	Error     *ocError `json:"error"`
	Text      string   `json:"text"`
	Tool      string   `json:"tool"`
	Name      string   `json:"name"`
	Title     string   `json:"title"`
}

type ocError struct {
	Name string `json:"name"`
	Data struct {
		Message    string `json:"message"`
		ProviderID string `json:"providerID"`
	} `json:"data"`
}

// message renders a human-readable error string, prefixing the provider when
// known (e.g. "kimi-for-coding: Anthropic API key is missing.").
func (e *ocError) message() string {
	if e == nil {
		return ""
	}
	msg := firstNonEmpty(e.Data.Message, e.Name)
	if e.Data.ProviderID != "" && msg != "" {
		return e.Data.ProviderID + ": " + msg
	}
	return msg
}

// mapEvent normalizes one raw opencode JSON line into a RunEvent.
func mapEvent(line []byte) RunEvent {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return RunEvent{Kind: EventLog}
	}
	var e ocEvent
	if err := json.Unmarshal(line, &e); err != nil {
		// Not JSON (formatted log leakage etc.) — surface it verbatim.
		return RunEvent{Kind: EventLog, Text: string(line), Raw: string(line)}
	}
	switch e.Type {
	case "error":
		msg := e.Error.message()
		return RunEvent{Kind: EventError, Err: msg, Text: msg, Raw: string(line)}
	case "tool", "tool.invoked", "tool_use", "tool.start":
		return RunEvent{Kind: EventTool, Text: firstNonEmpty(e.Tool, e.Name, e.Type), Raw: string(line)}
	case "subagent", "agent", "session.child", "task":
		return RunEvent{Kind: EventAgent, Text: firstNonEmpty(e.Name, e.Title, e.Type), Raw: string(line)}
	default:
		return RunEvent{Kind: EventLog, Text: firstNonEmpty(e.Text, e.Title, e.Type), Raw: string(line)}
	}
}
