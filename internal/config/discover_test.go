package config

import (
	"os"
	"testing"
)

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

func TestSplitKV(t *testing.T) {
	cases := []struct {
		in     string
		wantK  string
		wantV  string
		wantOK bool
	}{
		{"name: foo", "name", "foo", true},
		{"name:  \"quoted\"", "name", "\"quoted\"", true},
		{"description: a b c", "description", "a b c", true},
		{"no-colon", "", "", false},
		{"only:", "only", "", true},
	}
	for _, c := range cases {
		k, v, ok := splitKV(c.in)
		if ok != c.wantOK {
			t.Errorf("splitKV(%q) ok=%v want %v", c.in, ok, c.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if k != c.wantK || v != c.wantV {
			t.Errorf("splitKV(%q) = (%q, %q) want (%q, %q)", c.in, k, v, c.wantK, c.wantV)
		}
	}
}

func TestUnquote(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"hello"`, "hello"},
		{`'hello'`, "hello"},
		{`hello`, "hello"},
		{`"unmatched`, `"unmatched`},
		{`  "spaced"  `, "spaced"},
	}
	for _, c := range cases {
		if got := unquote(c.in); got != c.want {
			t.Errorf("unquote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseFrontmatter(t *testing.T) {
	content := `---
name: "My Skill"
description: 'A useful skill'
---
# Body
`
	dir := t.TempDir()
	path := dir + "/SKILL.md"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	name, desc, err := parseFrontmatter(path)
	if err != nil {
		t.Fatalf("parseFrontmatter: %v", err)
	}
	if name != "My Skill" || desc != "A useful skill" {
		t.Fatalf("parseFrontmatter = (%q, %q), want (My Skill, A useful skill)", name, desc)
	}
}

func TestParseFrontmatterMissing(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/SKILL.md"
	if err := os.WriteFile(path, []byte("# No frontmatter\n"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	_, _, err := parseFrontmatter(path)
	if err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

func TestDiscoverAgentsNoBin(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	agents, err := DiscoverAgents("")
	if err != nil {
		t.Fatalf("DiscoverAgents: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("expected 0 agents when no opencode is available, got %d", len(agents))
	}
}

func TestDiscoverModelsNoBin(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	models, err := DiscoverModels("")
	if err != nil {
		t.Fatalf("DiscoverModels: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("expected 0 models when no opencode is available, got %d", len(models))
	}
}

func TestResolveBinEmpty(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	if got := resolveBin(""); got != "" {
		t.Fatalf("expected empty resolveBin with no PATH/opencode, got %q", got)
	}
}

func TestResolveBinConfigured(t *testing.T) {
	tmp := t.TempDir() + "/opencode"
	if err := os.WriteFile(tmp, []byte("#!/bin/sh\necho mock\n"), 0o755); err != nil {
		t.Fatalf("write mock opencode: %v", err)
	}
	if got := resolveBin(tmp); got != tmp {
		t.Fatalf("resolveBin configured: got %q, want %q", got, tmp)
	}
}

func TestFileExists(t *testing.T) {
	f := t.TempDir() + "/file.txt"
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if !fileExists(f) {
		t.Fatal("fileExists returned false for existing file")
	}
	if fileExists(t.TempDir() + "/missing") {
		t.Fatal("fileExists returned true for missing file")
	}
	if fileExists(t.TempDir()) {
		t.Fatal("fileExists returned true for directory")
	}
}
