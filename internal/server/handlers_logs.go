package server

import (
	"net/http"

	"github.com/danieljustus/symaira-vibecoder/internal/engine"
)

// logsResp is the response shape for GET /api/logs: the current run id plus
// the bounded log/error replay buffer (oldest first).
type logsResp struct {
	RunID   string         `json:"run_id,omitempty"`
	Entries []engine.Event `json:"entries"`
}

// getLogs returns the engine's bounded log ring buffer so a reconnecting
// client can backfill log/error events it missed while disconnected. The
// buffer is memory-only and bounded (500 entries, matching the clients' own
// caps); the run ledger remains the durable record. Clients merge the entries
// into their local history by the ts field.
func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	entries := s.eng.Bus().Logs()
	if entries == nil {
		entries = []engine.Event{}
	}
	writeOK(w, logsResp{RunID: s.eng.State().RunID, Entries: entries})
}
