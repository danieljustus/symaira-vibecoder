package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Discovery populates the GUI pickers: available skills, opencode agents, and
// models. It degrades gracefully — if opencode is absent, agent/model discovery
// returns empty slices (skills are read straight from disk and still work).

// Skill is a discovered opencode skill (folder under skills/<name>/SKILL.md).
type Skill struct {
	Name        string `json:"name"`        // folder name == command name
	Description string `json:"description"` // from SKILL.md frontmatter
	Path        string `json:"path"`
}

// Agent / ModelInfo are discovered from the opencode binary.
type Agent struct {
	Name        string `json:"name"`                  // lowercase invocation name for --agent
	Description string `json:"description,omitempty"` // from the agent list header
	Role        string `json:"role,omitempty"`        // primary | subagent
}
type ModelInfo struct {
	ID string `json:"id"` // "provider/model"
}

// OpencodeSkillsDir is the conventional location of opencode skills.
func OpencodeSkillsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "skills")
}

// DiscoverSkills scans ~/.config/opencode/skills/*/SKILL.md and parses the
// YAML frontmatter (name + description) for each.
func DiscoverSkills() ([]Skill, error) {
	dir := OpencodeSkillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // graceful: no opencode skills installed
		}
		return nil, err
	}
	var out []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name(), "SKILL.md")
		name, desc, err := parseFrontmatter(p)
		if err != nil {
			continue // skip folders without a readable SKILL.md
		}
		if name == "" {
			name = e.Name() // fall back to folder name
		}
		out = append(out, Skill{Name: name, Description: desc, Path: p})
	}
	return out, nil
}

// parseFrontmatter extracts name and description from a SKILL.md YAML front-
// matter block delimited by leading and trailing "---" lines. Minimal parser:
// only the two scalar keys we need, tolerant of quotes.
func parseFrontmatter(path string) (name, desc string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	inFM := false
	started := false
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)
		if trim == "---" {
			if !started {
				started, inFM = true, true
				continue
			}
			break // end of frontmatter
		}
		if !inFM {
			continue
		}
		if k, v, ok := splitKV(trim); ok {
			switch k {
			case "name":
				name = unquote(v)
			case "description":
				desc = unquote(v)
			}
		}
	}
	if !started {
		return "", "", fmt.Errorf("%s: no frontmatter", path)
	}
	return name, desc, sc.Err()
}

func splitKV(s string) (k, v string, ok bool) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// DiscoverAgents runs `opencode agent list` to populate the agent picker.
// Returns nil (no error) when opencode is unavailable — graceful degradation.
//
// The output interleaves agent header lines at column 0 with indented (and some
// column-0 bare-bracket) JSON permission blocks, e.g.:
//
//	Sisyphus - ultraworker (primary)
//	  [ { "permission": "*", ... } ]
//	build (subagent)
//	  [ ... ]
//
// Only the header lines are agents; everything else is skipped.
func DiscoverAgents(opencodeBin string) ([]Agent, error) {
	bin := resolveBin(opencodeBin)
	if bin == "" {
		return nil, nil
	}
	out, err := exec.Command(bin, "agent", "list").Output()
	if err != nil {
		return nil, nil // degrade: picker just shows the configured default
	}
	var agents []Agent
	seen := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		raw := sc.Text()
		// Header lines start at column 0; indented lines are JSON block bodies.
		if raw == "" || raw[0] == ' ' || raw[0] == '\t' {
			continue
		}
		if a, ok := parseAgentHeader(raw); ok && !seen[a.Name] {
			seen[a.Name] = true
			agents = append(agents, a)
		}
	}
	return agents, nil
}

// parseAgentHeader parses a column-0 `agent list` header line of the form
// "Sisyphus - ultraworker (primary)" or "build (subagent)" into an Agent. It
// rejects stray JSON punctuation lines ("]", "{", `"pattern": ...`). The name
// is lowercased to match the key accepted by `opencode run --agent`.
func parseAgentHeader(line string) (Agent, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Agent{}, false
	}
	switch line[0] { // reject JSON fragments
	case '[', ']', '{', '}', '"', ',', ':':
		return Agent{}, false
	}
	role := ""
	if i := strings.LastIndex(line, "("); i >= 0 && strings.HasSuffix(line, ")") {
		role = strings.TrimSuffix(line[i+1:], ")")
		line = strings.TrimSpace(line[:i])
	}
	name, desc := line, ""
	if i := strings.Index(line, " - "); i >= 0 {
		name = strings.TrimSpace(line[:i])
		desc = strings.TrimSpace(line[i+3:])
	}
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return Agent{}, false
	}
	return Agent{Name: strings.ToLower(fields[0]), Description: desc, Role: role}, true
}

// DiscoverModels runs `opencode models` to populate the model picker. opencode
// prints one "provider/model" id per line. Graceful degradation as above.
func DiscoverModels(opencodeBin string) ([]ModelInfo, error) {
	bin := resolveBin(opencodeBin)
	if bin == "" {
		return nil, nil
	}
	out, err := exec.Command(bin, "models").Output()
	if err != nil {
		return nil, nil
	}
	var models []ModelInfo
	// Some opencode builds emit JSON; try that first, else line-per-id.
	var asJSON []string
	if json.Unmarshal(out, &asJSON) == nil && len(asJSON) > 0 {
		for _, id := range asJSON {
			models = append(models, ModelInfo{ID: id})
		}
		return models, nil
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		id := strings.TrimSpace(sc.Text())
		if id != "" && strings.Contains(id, "/") {
			models = append(models, ModelInfo{ID: id})
		}
	}
	return models, nil
}

// resolveBin returns a usable opencode path: the configured one (expanded),
// else PATH lookup, else the conventional ~/.opencode/bin/opencode, else "".
func resolveBin(configured string) string {
	if configured != "" {
		if p := expandHome(configured); fileExists(p) {
			return p
		}
	}
	if p, err := exec.LookPath("opencode"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	if p := filepath.Join(home, ".opencode", "bin", "opencode"); fileExists(p) {
		return p
	}
	return ""
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
