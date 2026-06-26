package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Template is the exportable, shareable wrapper for a Cycle definition.
type Template struct {
	Kind          string           `json:"kind"` // must be "symvibe.template"
	SchemaVersion int              `json:"schema_version"`
	Manifest      TemplateManifest `json:"manifest"`
	Requires      TemplateRequires `json:"requires"`
	Phases        []Phase          `json:"phases"`
}

// TemplateManifest holds metadata about the template.
type TemplateManifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
}

// TemplateRequires lists the categories, skills, agents, and sensors required
// for this template to run successfully on a local Baukasten instance.
type TemplateRequires struct {
	Skills     []string `json:"skills,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Agents     []string `json:"agents,omitempty"`
	Sensors    []string `json:"sensors,omitempty"`
}

// Catalog lists the local capabilities for validation.
type Catalog struct {
	Skills     []string `json:"skills"`
	Categories []string `json:"categories"`
	Agents     []string `json:"agents"`
	Sensors    []string `json:"sensors"`
}

// MissingRequirements holds resources required by a template but missing locally.
type MissingRequirements struct {
	Skills     []string `json:"skills,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Agents     []string `json:"agents,omitempty"`
	Sensors    []string `json:"sensors,omitempty"`
}

// Empty reports whether there are no missing requirements.
func (m *MissingRequirements) Empty() bool {
	return len(m.Skills) == 0 && len(m.Categories) == 0 && len(m.Agents) == 0 && len(m.Sensors) == 0
}

// ExportTemplate exports a Cycle as a Template.
func (c *Cycle) ExportTemplate(manifest TemplateManifest) *Template {
	def := c.Definition()

	skills := make(map[string]bool)
	categories := make(map[string]bool)
	agents := make(map[string]bool)
	sensors := make(map[string]bool)

	for _, p := range def.Phases {
		for _, s := range p.Steps {
			if s.Skill != "" {
				skills[s.Skill] = true
			}
			if s.Category != "" {
				categories[s.Category] = true
			}
			if s.Agent != "" {
				agents[s.Agent] = true
			}
			if s.AutoSkip != nil && s.AutoSkip.Sensor != "" {
				sensors[s.AutoSkip.Sensor] = true
			}
		}
	}

	req := TemplateRequires{
		Skills:     mapToSortedSlice(skills),
		Categories: mapToSortedSlice(categories),
		Agents:     mapToSortedSlice(agents),
		Sensors:    mapToSortedSlice(sensors),
	}

	if manifest.ID == "" {
		manifest.ID = def.ID
	}
	if manifest.Name == "" {
		manifest.Name = def.Name
	}
	if manifest.Description == "" {
		manifest.Description = def.Description
	}

	return &Template{
		Kind:          "symvibe.template",
		SchemaVersion: def.SchemaVersion,
		Manifest:      manifest,
		Requires:      req,
		Phases:        def.Phases,
	}
}

// ApplyRemap applies category mapping to the template's phases and steps,
// and regenerates the Category requirements list.
func (t *Template) ApplyRemap(remap map[string]string) {
	if len(remap) == 0 {
		return
	}
	for pi := range t.Phases {
		for si := range t.Phases[pi].Steps {
			s := &t.Phases[pi].Steps[si]
			if newCat, ok := remap[s.Category]; ok && newCat != "" {
				s.Category = newCat
			}
		}
	}

	// Re-derive categories requirement list
	categories := make(map[string]bool)
	for _, p := range t.Phases {
		for _, s := range p.Steps {
			if s.Category != "" {
				categories[s.Category] = true
			}
		}
	}
	t.Requires.Categories = mapToSortedSlice(categories)
}

// ImportTemplate parses JSON template data and returns a new Cycle.
func ImportTemplate(data []byte) (*Cycle, error) {
	var t Template
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("decode template: %w", err)
	}
	return ImportTemplateStruct(&t)
}

// ImportTemplateStruct instantiates a new Cycle from a parsed Template.
func ImportTemplateStruct(t *Template) (*Cycle, error) {
	if t.Kind != "symvibe.template" {
		return nil, fmt.Errorf("invalid template kind %q (expected %q)", t.Kind, "symvibe.template")
	}

	c := &Cycle{
		SchemaVersion: t.SchemaVersion,
		Name:          t.Manifest.Name,
		Description:   t.Manifest.Description,
		Phases:        t.Phases,
	}

	baseID := slugify(t.Manifest.ID)
	if baseID == "" {
		baseID = slugify(t.Manifest.Name)
	}

	id := baseID
	dir := CyclesDir()
	for n := 1; ; n++ {
		path := filepath.Join(dir, id+".toml")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		id = fmt.Sprintf("%s-%d", baseID, n)
	}
	c.ID = id

	c.Reindex()
	for pi := range c.Phases {
		for si := range c.Phases[pi].Steps {
			c.Phases[pi].Steps[si].Status = StatusPending
		}
	}

	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("imported cycle is invalid: %w", err)
	}

	return c, nil
}

// CheckRequirements matches template requirements against local catalog.
func CheckRequirements(t *Template, cat Catalog) *MissingRequirements {
	missing := &MissingRequirements{}

	hasSkill := make(map[string]bool)
	for _, s := range cat.Skills {
		hasSkill[s] = true
	}
	hasCategory := make(map[string]bool)
	for _, c := range cat.Categories {
		hasCategory[c] = true
	}
	hasAgent := make(map[string]bool)
	for _, a := range cat.Agents {
		hasAgent[strings.ToLower(a)] = true
	}
	hasSensor := make(map[string]bool)
	for _, s := range cat.Sensors {
		hasSensor[s] = true
	}

	for _, s := range t.Requires.Skills {
		if !hasSkill[s] {
			missing.Skills = append(missing.Skills, s)
		}
	}
	for _, c := range t.Requires.Categories {
		if !hasCategory[c] {
			missing.Categories = append(missing.Categories, c)
		}
	}
	for _, a := range t.Requires.Agents {
		if !hasAgent[strings.ToLower(a)] {
			missing.Agents = append(missing.Agents, a)
		}
	}
	for _, s := range t.Requires.Sensors {
		if !hasSensor[s] {
			missing.Sensors = append(missing.Sensors, s)
		}
	}

	return missing
}

func mapToSortedSlice(m map[string]bool) []string {
	var s []string
	for k := range m {
		s = append(s, k)
	}
	slices.Sort(s)
	return s
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else if r == ' ' {
			sb.WriteRune('-')
		}
	}
	res := sb.String()
	if res == "" {
		res = "imported"
	}
	return res
}
