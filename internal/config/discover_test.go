package config

import "testing"

func TestParseAgentHeader(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantOK   bool
	}{
		// Real `opencode agent list` header lines.
		{"Sisyphus - ultraworker (primary)", "sisyphus", true},
		{"Hephaestus - Deep Agent (primary)", "hephaestus", true},
		{"build (subagent)", "build", true},
		{"multimodal-looker (subagent)", "multimodal-looker", true},
		{"Sisyphus-Junior (subagent)", "sisyphus-junior", true},
		// JSON-block lines that must be rejected.
		{"]", "", false},
		{"[", "", false},
		{`  "permission": "*",`, "", false},
		{"{", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		a, ok := parseAgentHeader(c.in)
		if ok != c.wantOK {
			t.Errorf("%q: ok=%v want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && a.Name != c.wantName {
			t.Errorf("%q: name=%q want %q", c.in, a.Name, c.wantName)
		}
	}
}

func TestParseAgentHeaderRoleAndDesc(t *testing.T) {
	a, ok := parseAgentHeader("Prometheus - Plan Builder (primary)")
	if !ok || a.Name != "prometheus" || a.Description != "Plan Builder" || a.Role != "primary" {
		t.Fatalf("got %+v ok=%v", a, ok)
	}
}
