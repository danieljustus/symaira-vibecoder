package server

import (
	"strings"
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/runner"
)

func TestBuildHints(t *testing.T) {
	cases := []struct {
		name       string
		backend    string
		opencodeOK bool
		info       runner.Info
		gitOK      bool
		ghOK       bool
		wantKeys   []string
		wantSub    string
	}{
		{
			name:       "opencode missing",
			backend:    "opencode",
			opencodeOK: false,
			info:       runner.Info{Name: "opencode"},
			gitOK:      true,
			ghOK:       true,
			wantKeys:   []string{"opencode"},
		},
		{
			name:       "opencode too old",
			backend:    "opencode",
			opencodeOK: false,
			info:       runner.Info{Name: "opencode", Detail: "version 1.16.0 is older than required 1.17.0"},
			gitOK:      true,
			ghOK:       true,
			wantKeys:   []string{"opencode"},
			wantSub:    "upgrade opencode",
		},
		{
			name:       "api backend missing key",
			backend:    "api",
			opencodeOK: false,
			info:       runner.Info{Name: "api"},
			gitOK:      true,
			ghOK:       true,
			wantKeys:   []string{"api"},
			wantSub:    "SYMVIBE_ANTHROPIC_API_KEY",
		},
		{
			name:       "git and gh missing",
			backend:    "opencode",
			opencodeOK: true,
			info:       runner.Info{Name: "opencode"},
			gitOK:      false,
			ghOK:       false,
			wantKeys:   []string{"git", "gh"},
			wantSub:    "optional",
		},
		{
			name:       "all ok",
			backend:    "opencode",
			opencodeOK: true,
			info:       runner.Info{Name: "opencode"},
			gitOK:      true,
			ghOK:       true,
			wantKeys:   nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hints := buildHints(c.backend, c.opencodeOK, c.info, c.gitOK, c.ghOK)
			if c.wantKeys == nil {
				if hints != nil && len(hints) > 0 {
					t.Fatalf("expected no hints, got %v", hints)
				}
				return
			}
			for _, k := range c.wantKeys {
				if _, ok := hints[k]; !ok {
					t.Fatalf("expected hint key %q, got %v", k, hints)
				}
			}
			if c.wantSub != "" {
				found := false
				for _, v := range hints {
					if strings.Contains(v, c.wantSub) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected hint containing %q, got %v", c.wantSub, hints)
				}
			}
		})
	}
}
