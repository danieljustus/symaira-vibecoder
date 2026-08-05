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

// getLogs returns the engine's bounded log ring buffer so a reconnecting client
// can backfill log/error events it missed while disconnected.
func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	writeOK(w, logsResp{RunID: s.eng.State().RunID, Entries: s.eng.Bus().Logs()})
}
