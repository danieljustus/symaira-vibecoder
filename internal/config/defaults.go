package config

import "embed"

// seedFS embeds the shipped seed Baukasten so symvibe can materialize
// ~/.local/share/symvibe/cycles/default.toml on first run even when run from an
// arbitrary directory. (config/seed-cycle.toml lives at the repo root /config.)
//
//go:embed all:seed
var seedFS embed.FS // build tag: the seed copy is placed under internal/config/seed/ at build time

// SeedCycleName is the id of the shipped default Baukasten.
const SeedCycleName = "default"

// Default returns the built-in baseline Config. This is what symvibe runs on
// with no config.toml present: opencode backend, mirror of oh-my-openagent
// category bindings, GUI on 127.0.0.1:4317.
func Default() *Config {
	f := func(v float64) *float64 { return &v }
	s := func(v string) *string { return &v }
	sl := func(v ...string) *[]string { return &v }

	return &Config{
		SchemaVersion: 1,
		Server: ServerConfig{
			Host:        "127.0.0.1",
			Port:        4317,
			OpenBrowser: true,
			Access:      "loopback",
		},
		Runner: RunnerConfig{
			Backend:         "opencode",
			OpencodeBin:     "", // auto-detect on PATH; falls back to ~/.opencode/bin/opencode
			WorkingDir:      "",
			Mode:            "run", // MVP: one-shot `opencode run --format json` subprocess
			ServePort:       0,
			ServeHost:       "127.0.0.1",
			SkipPermissions: true,                    // unattended runs must not block on a permission prompt
			RequestTimeout:  Duration(30 * 60 * 1e9), // 30m
			MaxParallel:     4,
		},
		// Registry mirrors the distinct models used by oh-my-openagent.json.
		Models: map[string]Model{
			"mimo": {
				ID:             "opencode-go/mimo-v2.5",
				Temperature:    0.2,
				Variant:        "high",
				FallbackModels: []string{"opencode/big-pickle", "opencode-go/qwen3.7-plus", "opencode-go/deepseek-v4-flash"},
			},
			"flash": {
				ID:             "opencode-go/deepseek-v4-flash",
				Temperature:    0.2,
				FallbackModels: []string{"opencode/deepseek-v4-flash-free", "opencode/big-pickle"},
			},
			"qwen": {
				ID:             "opencode-go/qwen3.7-plus",
				Temperature:    0.25,
				Variant:        "high",
				FallbackModels: []string{"opencode-go/mimo-v2.5", "opencode-go/deepseek-v4-flash"},
			},
			"kimi": {
				ID:             "kimi-for-coding/k2p7",
				Temperature:    0.25,
				Variant:        "high",
				FallbackModels: []string{"opencode-go/minimax-m3", "opencode-go/mimo-v2.5"},
			},
		},
		// Category bindings mirror oh-my-openagent.json category names.
		Categories: map[string]CategoryBinding{
			"ultrabrain":         {ModelRef: "mimo"},
			"deep":               {ModelRef: "mimo", FallbackModels: sl("opencode/nemotron-3-ultra-free", "opencode-go/qwen3.7-plus", "opencode-go/deepseek-v4-flash")},
			"quick":              {ModelRef: "flash"},
			"writing":            {ModelRef: "flash", Temperature: f(0.6)},
			"git":                {ModelRef: "flash", Temperature: f(0.1)},
			"unspecified-low":    {ModelRef: "flash", Temperature: f(0.25)},
			"unspecified-high":   {ModelRef: "qwen"},
			"artistry":           {ModelRef: "mimo", Temperature: f(0.6), Variant: s("")},
			"visual-engineering": {ModelRef: "mimo", Temperature: f(0.35), Variant: s("")},
		},
		Defaults: Defaults{
			Category: "unspecified-high",
			// Empty => let opencode pick its default PRIMARY agent. Do not name a
			// subagent here (e.g. "build"): opencode rejects subagents as the
			// top-level --agent and falls back with a warning. Per-step `agent`
			// in the cycle may name a primary agent (e.g. "sisyphus").
			Agent: "",
			Cycle: SeedCycleName,
		},
	}
}
