package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/engine"
)

// sse streams live engine events to the board as Server-Sent Events. Each event
// is written as `event: <type>\ndata: <json>\n\n`; a comment ping every 15s
// keeps the connection alive through proxies. This is the push channel that
// flips the status glyphs and feeds the activity log.
func (s *Server) sse(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	id, ch := s.eng.Bus().Subscribe()
	defer s.eng.Bus().Unsubscribe(id)

	send := func(ev engine.Event) bool {
		b, err := json.Marshal(ev)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, b); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// Prime the client with the current run state so a late subscriber is in sync.
	st := s.eng.State()
	send(engine.Event{Type: "run_state", RunID: st.RunID, StepID: st.CurrentStep, State: st.State})

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if !send(ev) {
				return
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
