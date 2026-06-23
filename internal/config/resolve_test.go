package config

import (
	"strings"
	"testing"
)

func testCfg() *Config {
	return &Config{
		Models: map[string]Model{
			"big":   {ID: "prov/big", Temperature: 0.2, Variant: "high", FallbackModels: []string{"prov/mid", "prov/small"}},
			"small": {ID: "prov/small", Temperature: 0.1},
		},
		Categories: map[string]CategoryBinding{
			"deep":  {ModelRef: "big"},
			"quick": {ModelRef: "small"},
			"def":   {ModelRef: "small"},
		},
		Defaults: Defaults{Category: "def"},
	}
}

func TestResolvePrecedence(t *testing.T) {
	r := NewResolver(testCfg())

	// 1) per-step override wins.
	ov := &Model{ID: "x/override", Temperature: 0.9, Variant: "max"}
	rm, err := r.Resolve(Step{Category: "deep", ModelOverride: ov})
	if err != nil || rm.Model != "x/override" || rm.Source != "override" {
		t.Fatalf("override should win, got %+v err=%v", rm, err)
	}

	// 2) category binding.
	rm, _ = r.Resolve(Step{Category: "deep"})
	if rm.Model != "prov/big" || rm.Source != "category:deep" {
		t.Fatalf("category resolution wrong: %+v", rm)
	}

	// 3) empty category -> default category.
	rm, _ = r.Resolve(Step{})
	if rm.Model != "prov/small" || rm.Source != "default:def" {
		t.Fatalf("default resolution wrong: %+v", rm)
	}
}

func TestFallbackChainWalk(t *testing.T) {
	r := NewResolver(testCfg())
	spec, rm, err := r.BuildRunSpec(Step{Category: "deep"}, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Model != "prov/big" {
		t.Fatalf("primary should be prov/big, got %s", spec.Model)
	}
	chain := rm.Chain()
	want := []string{"prov/big", "prov/mid", "prov/small"}
	if len(chain) != 3 || chain[0] != want[0] || chain[2] != want[2] {
		t.Fatalf("chain wrong: %v", chain)
	}
	// Walk the chain.
	s2, ok := NextAttempt(spec, rm)
	if !ok || s2.Model != "prov/mid" {
		t.Fatalf("attempt 2 should be prov/mid, got %s ok=%v", s2.Model, ok)
	}
	s3, ok := NextAttempt(s2, rm)
	if !ok || s3.Model != "prov/small" {
		t.Fatalf("attempt 3 should be prov/small, got %s ok=%v", s3.Model, ok)
	}
	if _, ok := NextAttempt(s3, rm); ok {
		t.Fatal("chain should be exhausted after 3 attempts")
	}
}

func TestConfigValidateRequiresLoopbackServerHost(t *testing.T) {
	for _, tc := range []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", false},
		{"192.168.1.10", false},
		{"", false},
	} {
		cfg := Default()
		cfg.Server.Host = tc.host
		err := cfg.Validate()
		if tc.want && err != nil {
			t.Errorf("host %q rejected: %v", tc.host, err)
		}
		if !tc.want && (err == nil || !strings.Contains(err.Error(), "loopback")) {
			t.Errorf("host %q error = %v, want loopback rejection", tc.host, err)
		}
	}
}
