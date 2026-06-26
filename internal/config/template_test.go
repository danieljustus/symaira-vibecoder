package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportTemplate(t *testing.T) {
	c := &Cycle{
		ID:            "test-cycle",
		Name:          "Test Cycle",
		Description:   "A test cycle",
		SchemaVersion: 1,
		Phases: []Phase{
			{
				ID:   "p1",
				Name: "Phase 1",
				Steps: []Step{
					{
						ID:       "s1",
						Name:     "Step 1",
						Status:   StatusDone,
						Skill:    "git-commit",
						Category: "fast",
						Agent:    "Sisyphus",
						AutoSkip: &AutoSkip{
							Sensor: "git-dirty",
							When:   "clean",
						},
					},
					{
						ID:       "s2",
						Name:     "Step 2",
						Status:   StatusFailed,
						Skill:    "lint",
						Category: "deep",
					},
				},
			},
		},
	}

	manifest := TemplateManifest{
		Version: "2.1.0",
		Author:  "John Doe",
		Tags:    []string{"git", "ci"},
	}

	tpl := c.ExportTemplate(manifest)

	if tpl.Kind != "symvibe.template" {
		t.Errorf("expected Kind to be 'symvibe.template', got %q", tpl.Kind)
	}
	if tpl.SchemaVersion != c.SchemaVersion {
		t.Errorf("expected SchemaVersion %d, got %d", c.SchemaVersion, tpl.SchemaVersion)
	}

	// Manifest checks
	if tpl.Manifest.ID != "test-cycle" {
		t.Errorf("expected manifest ID 'test-cycle', got %q", tpl.Manifest.ID)
	}
	if tpl.Manifest.Name != "Test Cycle" {
		t.Errorf("expected manifest Name 'Test Cycle', got %q", tpl.Manifest.Name)
	}
	if tpl.Manifest.Version != "2.1.0" {
		t.Errorf("expected manifest Version '2.1.0', got %q", tpl.Manifest.Version)
	}
	if tpl.Manifest.Author != "John Doe" {
		t.Errorf("expected manifest Author 'John Doe', got %q", tpl.Manifest.Author)
	}

	// Requires checks
	expectedSkills := []string{"git-commit", "lint"}
	if len(tpl.Requires.Skills) != 2 || tpl.Requires.Skills[0] != expectedSkills[0] || tpl.Requires.Skills[1] != expectedSkills[1] {
		t.Errorf("incorrect required skills: %v", tpl.Requires.Skills)
	}

	expectedCategories := []string{"deep", "fast"}
	if len(tpl.Requires.Categories) != 2 || tpl.Requires.Categories[0] != expectedCategories[0] || tpl.Requires.Categories[1] != expectedCategories[1] {
		t.Errorf("incorrect required categories: %v", tpl.Requires.Categories)
	}

	expectedAgents := []string{"Sisyphus"}
	if len(tpl.Requires.Agents) != 1 || tpl.Requires.Agents[0] != expectedAgents[0] {
		t.Errorf("incorrect required agents: %v", tpl.Requires.Agents)
	}

	expectedSensors := []string{"git-dirty"}
	if len(tpl.Requires.Sensors) != 1 || tpl.Requires.Sensors[0] != expectedSensors[0] {
		t.Errorf("incorrect required sensors: %v", tpl.Requires.Sensors)
	}

	// Verify status is stripped in phases
	if tpl.Phases[0].Steps[0].Status != StatusPending || tpl.Phases[0].Steps[1].Status != StatusPending {
		t.Errorf("exported template steps did not have status stripped: %+v", tpl.Phases[0].Steps)
	}
}

func TestImportTemplate(t *testing.T) {
	tmpDataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDataHome)

	// Ensure the cycles directory exists so ImportTemplate can check it
	cyclesDir := filepath.Join(tmpDataHome, "symvibe", "cycles")
	if err := os.MkdirAll(cyclesDir, 0755); err != nil {
		t.Fatalf("failed to create temp cycles dir: %v", err)
	}

	tpl := &Template{
		Kind:          "symvibe.template",
		SchemaVersion: 1,
		Manifest: TemplateManifest{
			ID:          "Awesome-Template",
			Name:        "Awesome Name",
			Description: "A great template",
		},
		Phases: []Phase{
			{
				ID:   "p1",
				Name: "P1",
				Steps: []Step{
					{
						ID:     "s1",
						Name:   "Step 1",
						Status: StatusDone,
					},
				},
			},
		},
	}

	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("failed to marshal template: %v", err)
	}

	// 1) First import
	c1, err := ImportTemplate(data)
	if err != nil {
		t.Fatalf("ImportTemplate failed: %v", err)
	}

	if c1.ID != "awesome-template" {
		t.Errorf("expected cycle ID 'awesome-template', got %q", c1.ID)
	}
	if c1.Phases[0].Steps[0].Status != StatusPending {
		t.Errorf("expected status pending, got %q", c1.Phases[0].Steps[0].Status)
	}

	// Simulate saving c1 to disk so next import detects it
	if err := SaveCycle(c1); err != nil {
		t.Fatalf("failed to save imported cycle: %v", err)
	}

	// 2) Second import (should generate suffix since awesome-template.toml now exists)
	c2, err := ImportTemplate(data)
	if err != nil {
		t.Fatalf("second ImportTemplate failed: %v", err)
	}

	if c2.ID != "awesome-template-1" {
		t.Errorf("expected cycle ID to get suffix, got %q", c2.ID)
	}
}

func TestCheckRequirementsAndRemap(t *testing.T) {
	tpl := &Template{
		Kind:          "symvibe.template",
		SchemaVersion: 1,
		Manifest: TemplateManifest{
			ID:   "t1",
			Name: "T1",
		},
		Requires: TemplateRequires{
			Skills:     []string{"git", "unsupported-skill"},
			Categories: []string{"fast", "missing-cat"},
			Agents:     []string{"sisyphus", "missing-agent"},
			Sensors:    []string{"git-dirty", "missing-sensor"},
		},
		Phases: []Phase{
			{
				ID:   "p1",
				Name: "P1",
				Steps: []Step{
					{
						ID:       "s1",
						Name:     "S1",
						Category: "missing-cat",
					},
				},
			},
		},
	}

	cat := Catalog{
		Skills:     []string{"git", "lint"},
		Categories: []string{"fast", "deep"},
		Agents:     []string{"sisyphus"},
		Sensors:    []string{"git-dirty", "open-issues"},
	}

	// Check initially
	missing := CheckRequirements(tpl, cat)
	if len(missing.Skills) != 1 || missing.Skills[0] != "unsupported-skill" {
		t.Errorf("incorrect missing skills: %v", missing.Skills)
	}
	if len(missing.Categories) != 1 || missing.Categories[0] != "missing-cat" {
		t.Errorf("incorrect missing categories: %v", missing.Categories)
	}
	if len(missing.Agents) != 1 || missing.Agents[0] != "missing-agent" {
		t.Errorf("incorrect missing agents: %v", missing.Agents)
	}
	if len(missing.Sensors) != 1 || missing.Sensors[0] != "missing-sensor" {
		t.Errorf("incorrect missing sensors: %v", missing.Sensors)
	}

	// Apply remap for missing category
	tpl.ApplyRemap(map[string]string{
		"missing-cat": "deep",
	})

	// Check again
	missing2 := CheckRequirements(tpl, cat)
	if len(missing2.Categories) != 0 {
		t.Errorf("expected no missing categories after remap, got %v", missing2.Categories)
	}

	// Verify step category was updated
	if tpl.Phases[0].Steps[0].Category != "deep" {
		t.Errorf("expected step category to be updated to 'deep', got %q", tpl.Phases[0].Steps[0].Category)
	}
}
