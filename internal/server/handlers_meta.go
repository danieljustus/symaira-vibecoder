package server

import (
	"net/http"
	"os/exec"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

// getSkills lists the opencode skills available to bind to steps. Returns an
// empty list (not an error) when opencode/skills are absent.
func (s *Server) getSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := config.DiscoverSkills()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if skills == nil {
		skills = []config.Skill{}
	}
	writeOK(w, skills)
}

type modelsResp struct {
	Registry        map[string]config.Model           `json:"registry"`
	Categories      map[string]config.CategoryBinding `json:"categories"`
	DefaultCategory string                            `json:"default_category"`
	Discovered      []config.ModelInfo                `json:"discovered"`
	Agents          []config.Agent                    `json:"agents"`
}

// getModels returns the registry, category bindings, the default category, and
// the live model/agent ids discovered from opencode (for the GUI pickers).
func (s *Server) getModels(w http.ResponseWriter, r *http.Request) {
	discovered, _ := config.DiscoverModels(s.cfg.Runner.OpencodeBin)
	agents, _ := config.DiscoverAgents(s.cfg.Runner.OpencodeBin)
	if discovered == nil {
		discovered = []config.ModelInfo{}
	}
	if agents == nil {
		agents = []config.Agent{}
	}
	writeOK(w, modelsResp{
		Registry:        s.cfg.Models,
		Categories:      s.cfg.Categories,
		DefaultCategory: s.cfg.Defaults.Category,
		Discovered:      discovered,
		Agents:          agents,
	})
}

// getCategories returns just the category bindings + the default (lighter than
// /api/models for the per-step category dropdown).
func (s *Server) getCategories(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{
		"categories":       s.cfg.Categories,
		"default_category": s.cfg.Defaults.Category,
	})
}

type doctorResp struct {
	Opencode   runner.Info `json:"opencode"`
	OpencodeOK bool        `json:"opencode_ok"`
	Git        bool        `json:"git"`
	Gh         bool        `json:"gh"`
	Runnable   bool        `json:"runnable"`
}

// getDoctor reports backend availability. The board uses opencode_ok/runnable to
// enable or grey out the run controls (graceful degradation).
func (s *Server) getDoctor(w http.ResponseWriter, r *http.Request) {
	ok, info := s.eng.Available(r.Context())
	writeOK(w, doctorResp{
		Opencode:   info,
		OpencodeOK: ok,
		Git:        onPath("git"),
		Gh:         onPath("gh"),
		Runnable:   ok,
	})
}

func onPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}
