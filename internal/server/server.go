// Package server exposes the Baukasten board over HTTP: a small REST API for
// reading and editing the cycle and controlling runs, a single Server-Sent
// Events stream for live status, and the embedded web UI. It binds to
// 127.0.0.1 by default; nothing here ever exposes opencode to the network.
package server

import (
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/danieljustus/symaira-vibecoder/internal/auth"
	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/engine"
)

// Server wires the engine + config + embedded UI into an http.Handler.
type Server struct {
	cfg   *config.Config
	eng   *engine.Engine
	dist  fs.FS
	mux   *http.ServeMux
	store auth.TokenStore
}

// New builds the server. dist is the embedded web/dist filesystem (rooted at the
// dist directory) serving the board.
func New(cfg *config.Config, eng *engine.Engine, dist fs.FS) *Server {
	s := &Server{cfg: cfg, eng: eng, dist: dist}
	s.routes()
	return s
}

func (s *Server) SetTokenStore(store auth.TokenStore) { s.store = store }

// Handler returns the root http.Handler (for http.Server / tests).
func (s *Server) Handler() http.Handler {
	if s.store == nil {
		return s.mux
	}
	bypass := s.cfg.Server.Access == "" || s.cfg.Server.Access == "loopback"
	return auth.Middleware(s.mux, s.store, bypass)
}

func (s *Server) routes() {
	m := http.NewServeMux()

	// Cycle (Baukasten) read + edit.
	m.HandleFunc("GET /api/cycle", s.getCycle)
	m.HandleFunc("PUT /api/cycle", s.putCycle)
	m.HandleFunc("POST /api/cycle/step", s.addStep)
	m.HandleFunc("DELETE /api/cycle/step/{id}", s.deleteStep)
	m.HandleFunc("POST /api/cycle/step/{id}/move", s.moveStep)
	m.HandleFunc("POST /api/cycle/step/{id}/duplicate", s.duplicateStep)
	m.HandleFunc("POST /api/cycle/phase", s.addPhase)
	m.HandleFunc("DELETE /api/cycle/phase/{id}", s.deletePhase)

	// Discovery / config surfaces for the GUI pickers.
	m.HandleFunc("GET /api/skills", s.getSkills)
	m.HandleFunc("GET /api/models", s.getModels)
	m.HandleFunc("GET /api/categories", s.getCategories)
	m.HandleFunc("GET /api/doctor", s.getDoctor)

	// Run control.
	m.HandleFunc("GET /api/runstate", s.getRunState)
	m.HandleFunc("POST /api/run", s.runCycle)
	m.HandleFunc("POST /api/run/step", s.runStep)
	m.HandleFunc("POST /api/run/control", s.runControl)

	// Live status stream.
	m.HandleFunc("GET /events", s.sse)

	// Embedded board (SPA) + assets — least specific, matches everything else.
	m.HandleFunc("GET /", s.static)

	s.mux = m
}

// busy reports whether a run is active (board edits are rejected during a run to
// avoid mutating a cycle the engine is walking).
func (s *Server) busy() bool { return s.eng.State().State != "idle" }

// static serves the embedded board with SPA fallback to index.html.
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "index.html"
	}
	if f, err := s.dist.Open(p); err == nil {
		_ = f.Close()
		http.FileServerFS(s.dist).ServeHTTP(w, r)
		return
	}
	idx, err := s.dist.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer idx.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, idx)
}
