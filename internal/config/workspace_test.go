package config

import "testing"

func TestWorkspaceModeIsValid(t *testing.T) {
	if !WorkspaceShared.IsValid() {
		t.Fatal("shared should be valid")
	}
	if !WorkspaceIsolated.IsValid() {
		t.Fatal("isolated should be valid")
	}
	if WorkspaceMode("unknown").IsValid() {
		t.Fatal("unknown mode should be invalid")
	}
	if WorkspaceMode("").IsValid() {
		t.Fatal("empty mode should be invalid")
	}
}

func TestWorkspaceConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     WorkspaceConfig
		wantErr bool
	}{
		{"empty (zero value)", WorkspaceConfig{}, false},
		{"shared explicit", WorkspaceConfig{Mode: WorkspaceShared}, false},
		{"isolated", WorkspaceConfig{Mode: WorkspaceIsolated}, false},
		{"isolated with worktree dir", WorkspaceConfig{Mode: WorkspaceIsolated, WorktreeDir: ".my-worktrees"}, false},
		{"unknown mode", WorkspaceConfig{Mode: "hybrid"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWorkspaceConfigEffectiveMode(t *testing.T) {
	var emptyCfg WorkspaceConfig
	if emptyCfg.EffectiveMode() != WorkspaceShared {
		t.Fatal("empty config should default to shared")
	}
	sharedCfg := WorkspaceConfig{Mode: WorkspaceShared}
	if sharedCfg.EffectiveMode() != WorkspaceShared {
		t.Fatal("explicit shared should stay shared")
	}
	isoCfg := WorkspaceConfig{Mode: WorkspaceIsolated}
	if isoCfg.EffectiveMode() != WorkspaceIsolated {
		t.Fatal("explicit isolated should stay isolated")
	}
}

func TestWorkspaceRoundTripInCycle(t *testing.T) {
	// Verify that WorkspaceConfig serializes/deserializes via TOML in a Cycle.
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cycle := &Cycle{
		ID:   "test-cycle",
		Name: "Test",
		Workspace: WorkspaceConfig{
			Mode:        WorkspaceIsolated,
			WorktreeDir: ".custom-worktrees",
		},
		Phases: []Phase{{
			ID: "p1", Name: "Phase 1", Order: 1,
			Steps: []Step{{ID: "1.1", Name: "Step 1", Order: 1, Enabled: true}},
		}},
	}

	if err := SaveCycle(cycle); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadCycle("test-cycle")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Workspace.EffectiveMode() != WorkspaceIsolated {
		t.Fatalf("mode = %q, want isolated", loaded.Workspace.EffectiveMode())
	}
	if loaded.Workspace.WorktreeDir != ".custom-worktrees" {
		t.Fatalf("worktree_dir = %q, want .custom-worktrees", loaded.Workspace.WorktreeDir)
	}
}
