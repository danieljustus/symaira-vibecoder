package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/danieljustus/symaira-vibecoder/internal/engine"
)

func (s *Server) getRunState(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.eng.State())
}

// requireRunnable returns false (and writes 503) when the backend can't run.
func (s *Server) requireRunnable(w http.ResponseWriter, r *http.Request) bool {
	ok, info := s.eng.Available(r.Context())
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":   "runner unavailable",
			"backend": info,
		})
		return false
	}
	return true
}

func (s *Server) runCycle(w http.ResponseWriter, r *http.Request) {
	if !s.requireRunnable(w, r) {
		return
	}
	runID, err := s.eng.StartCycle()
	s.writeRunStart(w, runID, err)
}

func (s *Server) runStep(w http.ResponseWriter, r *http.Request) {
	if !s.requireRunnable(w, r) {
		return
	}
	var body struct {
		StepID string `json:"step_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.StepID == "" {
		writeErr(w, http.StatusBadRequest, "step_id required")
		return
	}
	runID, err := s.eng.StartStep(body.StepID)
	s.writeRunStart(w, runID, err)
}

func (s *Server) writeRunStart(w http.ResponseWriter, runID string, err error) {
	switch {
	case errors.Is(err, engine.ErrBusy):
		writeErr(w, http.StatusConflict, err.Error())
	case err != nil:
		writeErr(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusAccepted, map[string]string{"run_id": runID})
	}
}

func (s *Server) runControl(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action string `json:"action"` // pause | resume | cancel
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	switch body.Action {
	case "cancel":
		s.eng.Cancel()
		writeOK(w, map[string]bool{"ok": true})
	case "pause":
		s.eng.Pause()
		writeOK(w, map[string]bool{"ok": true})
	case "resume":
		runID, err := s.eng.Resume()
		s.writeRunStart(w, runID, err)
	default:
		writeErr(w, http.StatusBadRequest, "unknown action (want pause|resume|cancel)")
	}
}
