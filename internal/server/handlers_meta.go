package server

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
	"github.com/danieljustus/symaira-vibecoder/internal/runner"
	"github.com/danieljustus/symaira-vibecoder/internal/version"
)

// capability is a named server feature exposed to clients for graceful
// degradation. Keep names stable: they are part of the public API contract.
type capability string

const (
	capRun          capability = "run"
	capEdit         capability = "edit"
	capPairing      capability = "pairing"
	capTLS          capability = "tls"
	capMulticastDNS capability = "mdns"
)

// versionResp is the response shape for GET /api/version.
type versionResp struct {
	APIVersion    string       `json:"api_version"`
	ServerVersion string       `json:"server_version"`
	Capabilities  []capability `json:"capabilities"`
	GoVersion     string       `json:"go_version"`
	Platform      string       `json:"platform"`
}

// getVersion reports the server's API contract version and capabilities so
// clients can degrade gracefully across server versions.
func (s *Server) getVersion(w http.ResponseWriter, r *http.Request) {
	caps := []capability{capRun, capEdit}
	if s.cfg.Auth.Enabled {
		caps = append(caps, capPairing)
	}
	if s.cfg.Server.Access == "lan" || s.cfg.Server.Access == "relay" {
		caps = append(caps, capTLS)
	}
	if s.cfg.Server.MulticastDNS {
		caps = append(caps, capMulticastDNS)
	}
	writeOK(w, versionResp{
		APIVersion:    version.APIVersion,
		ServerVersion: version.Version,
		Capabilities:  caps,
		GoVersion:     version.GoVersion(),
		Platform:      version.Platform(),
	})
}

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
	Opencode   runner.Info       `json:"opencode"`
	OpencodeOK bool              `json:"opencode_ok"`
	Git        bool              `json:"git"`
	Gh         bool              `json:"gh"`
	Runnable   bool              `json:"runnable"`
	Hints      map[string]string `json:"hints,omitempty"`
}

// getDoctor reports backend availability. The board uses opencode_ok/runnable to
// enable or grey out the run controls (graceful degradation).
func (s *Server) getDoctor(w http.ResponseWriter, r *http.Request) {
	ok, info := s.eng.Available(r.Context())
	gitOK := onPath("git")
	ghOK := onPath("gh")
	hints := buildHints(s.cfg.Runner.Backend, ok, info, gitOK, ghOK)
	writeOK(w, doctorResp{
		Opencode:   info,
		OpencodeOK: ok,
		Git:        gitOK,
		Gh:         ghOK,
		Runnable:   ok,
		Hints:      hints,
	})
}

func buildHints(backend string, opencodeOK bool, info runner.Info, gitOK, ghOK bool) map[string]string {
	hints := map[string]string{}
	switch backend {
	case "api":
		if !opencodeOK {
			hints["api"] = "backend=api is configured; set runner.api_key or SYMVIBE_ANTHROPIC_API_KEY to run steps without opencode"
		}
	default:
		if strings.Contains(info.Detail, "older than required") {
			hints["opencode"] = "upgrade opencode: curl -fsSL https://opencode.ai/install.sh | sh"
		} else if !opencodeOK {
			hints["opencode"] = "install opencode: curl -fsSL https://opencode.ai/install.sh | sh   (or brew install opencode if available)"
		}
	}
	if !gitOK {
		hints["git"] = "install git: https://git-scm.com/downloads"
	}
	if !ghOK {
		hints["gh"] = "install gh (optional): https://cli.github.com  — only needed for GitHub workflows"
	}
	if len(hints) == 0 {
		return nil
	}
	return hints
}

func onPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}
