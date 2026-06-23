package server

import (
	"encoding/json"
	"net/http"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/engine"
)

// loadCycle returns the active (default) cycle from disk, materializing the seed
// on first run.
func (s *Server) loadCycle() (*config.Cycle, error) {
	return config.LoadCycle(s.cfg.Defaults.Cycle)
}

// persist saves the cycle and notifies boards to refetch.
func (s *Server) persist(w http.ResponseWriter, c *config.Cycle) bool {
	c.Reindex()
	if err := c.Validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return false
	}
	if err := config.SaveCycle(c); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return false
	}
	s.eng.Bus().Publish(engine.Event{Type: "board"})
	return true
}

func (s *Server) getCycle(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, c)
}

// putCycle replaces the whole cycle (the simplest robust edit path: the board
// PUTs its full edited model).
func (s *Server) putCycle(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var c config.Cycle
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid cycle json: "+err.Error())
		return
	}
	if c.ID == "" {
		c.ID = s.cfg.Defaults.Cycle
	}
	if !s.persist(w, &c) {
		return
	}
	writeOK(w, &c)
}

func (s *Server) addStep(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var body struct {
		PhaseID string      `json:"phase_id"`
		Step    config.Step `json:"step"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, err := c.AddStep(body.PhaseID, body.Step)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]string{"id": id})
}

func (s *Server) deleteStep(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !c.DeleteStep(r.PathValue("id")) {
		writeErr(w, http.StatusNotFound, "step not found")
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

func (s *Server) moveStep(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var body struct {
		ToPhaseID string `json:"to_phase_id"`
		ToIndex   int    `json:"to_index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := c.MoveStep(r.PathValue("id"), body.ToPhaseID, body.ToIndex); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

func (s *Server) duplicateStep(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id, err := c.DuplicateStep(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]string{"id": id})
}

func (s *Server) addPhase(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "New Phase"
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	id := c.AddPhase(body.Name)
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]string{"id": id})
}

func (s *Server) deletePhase(w http.ResponseWriter, r *http.Request) {
	if s.busy() {
		writeErr(w, http.StatusConflict, "a run is in progress")
		return
	}
	c, err := s.loadCycle()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !c.DeletePhase(r.PathValue("id")) {
		writeErr(w, http.StatusNotFound, "phase not found")
		return
	}
	if !s.persist(w, c) {
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}
