package config

import (
	"testing"

	"github.com/BurntSushi/toml"
)

// TestEmbeddedSeedParses guards the shipped Baukasten: it must parse, validate,
// carry all 8 phases from docs/Grundidee.csv, and keep its auto_skip wiring.
func TestEmbeddedSeedParses(t *testing.T) {
	data, err := seedFS.ReadFile("seed/seed-cycle.toml")
	if err != nil {
		t.Fatal(err)
	}
	var c Cycle
	if err := toml.Unmarshal(data, &c); err != nil {
		t.Fatalf("seed does not parse: %v", err)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("seed fails validation: %v", err)
	}
	if len(c.Phases) != 8 {
		t.Fatalf("want 8 phases, got %d", len(c.Phases))
	}
	if _, s := c.FindStep("4.1"); s == nil || s.AutoSkip == nil || s.AutoSkip.Sensor != "open-issues" {
		t.Fatalf("step 4.1 auto_skip not wired: %+v", s)
	}
	// The per-step model override on 2.3 must survive the round trip.
	if _, s := c.FindStep("2.3"); s == nil || s.ModelOverride == nil || s.ModelOverride.Variant != "max" {
		t.Fatalf("step 2.3 model_override not parsed: %+v", s)
	}
}
