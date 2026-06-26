package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

func TestExportCycleHandler(t *testing.T) {
	s := newTestServer(t, true)

	// Seed cycle should be materialized on first read since it doesn't exist yet
	req := httptest.NewRequest("GET", "/api/cycle/export?id=test-cycle", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	// Since newTestServer sets defaults.cycle to "test-cycle", let's make sure it exists by saving a dummy first or using the default.
	// Actually, let's create and save a dummy cycle "test-cycle" to make sure it loads.
	dummy := &config.Cycle{
		ID:            "test-cycle",
		Name:          "Test Cycle",
		Description:   "Dummy",
		SchemaVersion: 1,
	}
	if err := config.SaveCycle(dummy); err != nil {
		t.Fatalf("failed to save dummy cycle: %v", err)
	}

	req = httptest.NewRequest("GET", "/api/cycle/export?id=test-cycle", nil)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected export to return 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	disposition := rr.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, `attachment; filename="test-cycle.json"`) {
		t.Errorf("expected Content-Disposition attachment, got %q", disposition)
	}

	var template config.Template
	if err := json.Unmarshal(rr.Body.Bytes(), &template); err != nil {
		t.Fatalf("failed to decode exported template: %v", err)
	}

	if template.Kind != "symvibe.template" {
		t.Errorf("expected template Kind 'symvibe.template', got %q", template.Kind)
	}
}

func TestImportCycleHandler(t *testing.T) {
	s := newTestServer(t, true)

	// Override server config properties for testing
	s.cfg.Categories = map[string]config.CategoryBinding{
		"fast": {ModelRef: "fast-model"},
		"deep": {ModelRef: "deep-model"},
	}
	s.cfg.Models = map[string]config.Model{
		"fast-model": {ID: "provider/fast"},
		"deep-model": {ID: "provider/deep"},
	}
	s.cfg.Defaults.Category = "fast"
	s.cfg.Defaults.Agent = "sisyphus"

	// Create cycles dir inside the test's temp data directory
	cyclesDir := filepath.Join(os.Getenv("XDG_DATA_HOME"), "symvibe", "cycles")
	if err := os.MkdirAll(cyclesDir, 0755); err != nil {
		t.Fatalf("failed to create cycles dir: %v", err)
	}

	template := &config.Template{
		Kind:          "symvibe.template",
		SchemaVersion: 1,
		Manifest: config.TemplateManifest{
			ID:          "test-import",
			Name:        "Test Import",
			Description: "Test Import Description",
		},
		Requires: config.TemplateRequires{
			Categories: []string{"fast"}, // Available locally
		},
		Phases: []config.Phase{
			{
				ID:   "p1",
				Name: "Phase 1",
				Steps: []config.Step{
					{
						ID:       "s1",
						Name:     "Step 1",
						Category: "fast",
					},
				},
			},
		},
	}

	// 1) Test valid import
	body, _ := json.Marshal(map[string]any{
		"template": template,
	})
	req := httptest.NewRequest("POST", "/api/cycle/import", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected valid import to return 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var importedCycle config.Cycle
	if err := json.Unmarshal(rr.Body.Bytes(), &importedCycle); err != nil {
		t.Fatalf("failed to decode imported cycle response: %v", err)
	}
	if importedCycle.ID != "test-import" {
		t.Errorf("expected ID 'test-import', got %q", importedCycle.ID)
	}

	// 2) Test missing requirements import
	template.Requires.Categories = []string{"missing-category"}
	template.Phases[0].Steps[0].Category = "missing-category"

	body, _ = json.Marshal(map[string]any{
		"template": template,
	})
	req = httptest.NewRequest("POST", "/api/cycle/import", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected missing requirements to return 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var missingResp struct {
		Error   string                     `json:"error"`
		Missing config.MissingRequirements `json:"missing"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &missingResp); err != nil {
		t.Fatalf("failed to decode missing response: %v", err)
	}
	if missingResp.Error != "missing requirements" {
		t.Errorf("expected error 'missing requirements', got %q", missingResp.Error)
	}
	if len(missingResp.Missing.Categories) != 1 || missingResp.Missing.Categories[0] != "missing-category" {
		t.Errorf("expected missing category 'missing-category', got %v", missingResp.Missing.Categories)
	}

	// 3) Test import with remap
	body, _ = json.Marshal(map[string]any{
		"template": template,
		"remap": map[string]string{
			"missing-category": "deep", // Remap to available "deep" category
		},
	})
	req = httptest.NewRequest("POST", "/api/cycle/import", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected import with remap to return 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var remappedCycle config.Cycle
	if err := json.Unmarshal(rr.Body.Bytes(), &remappedCycle); err != nil {
		t.Fatalf("failed to decode remapped cycle: %v", err)
	}

	// Should have resolved and been saved under a unique ID since test-import already exists
	if remappedCycle.ID != "test-import-1" {
		t.Errorf("expected ID 'test-import-1', got %q", remappedCycle.ID)
	}
	if remappedCycle.Phases[0].Steps[0].Category != "deep" {
		t.Errorf("expected remapped category to be 'deep', got %q", remappedCycle.Phases[0].Steps[0].Category)
	}
}
