// Package config loads, validates, and resolves symvibe configuration.
//
// Layering (lowest -> highest precedence):
//
//	built-in defaults (defaults.go)  <  ~/.config/symvibe/config.toml
//	                                 <  SYMVIBE_* env vars  <  CLI flags
//
// The Baukasten ("cycle") lives separately under the data dir so user edits in
// the GUI survive; see Cycle / LoadCycle / SaveCycle.
package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ---------------------------------------------------------------------------
// On-disk schema (config.toml)
// ---------------------------------------------------------------------------

// Config is the fully merged, validated configuration.
type Config struct {
	SchemaVersion int                        `toml:"schema_version"`
	Server        ServerConfig               `toml:"server"`
	Auth          AuthConfig                 `toml:"auth"`
	Runner        RunnerConfig               `toml:"runner"`
	Models        map[string]Model           `toml:"models"`     // registry: name -> model
	Categories    map[string]CategoryBinding `toml:"categories"` // category -> binding
	Defaults      Defaults                   `toml:"defaults"`
}

type ServerConfig struct {
	Host        string `toml:"host"`
	Port        int    `toml:"port"`
	OpenBrowser bool   `toml:"open_browser"`
	Access      string `toml:"access"` // loopback | lan | relay
}

type AuthConfig struct {
	Enabled bool `toml:"enabled"`
}

type RunnerConfig struct {
	Backend     string `toml:"backend"`      // opencode | claudecode | api
	OpencodeBin string `toml:"opencode_bin"` // empty -> auto-detect on PATH
	WorkingDir  string `toml:"working_dir"`
	Mode        string `toml:"mode"`       // serve | run (MVP default: run)
	ServePort   int    `toml:"serve_port"` // 0 -> free port
	ServeHost   string `toml:"serve_host"`
	// SkipPermissions passes `--dangerously-skip-permissions` to opencode so an
	// unattended step does not block on a tool-permission prompt (no TTY). See
	// SECURITY.md — this auto-approves any permission not explicitly denied.
	SkipPermissions bool     `toml:"skip_permissions"`
	RequestTimeout  Duration `toml:"request_timeout"`
	MaxParallel     int      `toml:"max_parallel_subagents"`
}

// Model is one entry in the registry. id is the opencode "provider/model" id.
type Model struct {
	ID             string   `toml:"id" json:"id"`
	Temperature    float64  `toml:"temperature" json:"temperature"`
	Variant        string   `toml:"variant" json:"variant"` // high | max | minimal | ""
	FallbackModels []string `toml:"fallback_models" json:"fallback_models"`
}

// CategoryBinding points a work-area at a registry model and may inline-override
// fields. A nil/zero override field means "inherit from the referenced Model".
type CategoryBinding struct {
	ModelRef       string    `toml:"model_ref" json:"model_ref"`
	Temperature    *float64  `toml:"temperature" json:"temperature,omitempty"`
	Variant        *string   `toml:"variant" json:"variant,omitempty"`
	FallbackModels *[]string `toml:"fallback_models" json:"fallback_models,omitempty"`
}

type Defaults struct {
	Category string `toml:"category"`
	Agent    string `toml:"agent"`
	Cycle    string `toml:"cycle"`
}

// Duration is a TOML-friendly time.Duration ("30m", "1h30m").
type Duration time.Duration

func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}
func (d Duration) Std() time.Duration { return time.Duration(d) }

// ---------------------------------------------------------------------------
// Resolution result (consumed by the Runner)
// ---------------------------------------------------------------------------

// ResolvedModel is the effective binding for a single step, after applying
// precedence (override > category > default) and registry inheritance. This is
// what gets mapped onto Runner per-call flags (--model / --variant).
type ResolvedModel struct {
	Model          string // opencode "provider/model" id, e.g. "opencode-go/mimo-v2.5"
	Temperature    float64
	Variant        string   // "" | high | max | minimal
	FallbackModels []string // tried in order on failure
	Source         string   // "override" | "category:<name>" | "default:<name>" — for GUI/debug
}

// Chain returns the full ordered attempt list: primary first, then fallbacks.
func (r ResolvedModel) Chain() []string {
	return append([]string{r.Model}, r.FallbackModels...)
}

// ---------------------------------------------------------------------------
// Loading
// ---------------------------------------------------------------------------

// Load reads config.toml from path (or the XDG default if path==""), merges it
// over built-in defaults, applies SYMVIBE_* env overrides, and validates.
func Load(path string) (*Config, error) {
	cfg := Default() // built-in baseline (defaults.go)

	if path == "" {
		path = filepath.Join(configDir(), "config.toml")
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	// Missing file is fine: defaults stand.

	applyEnv(cfg)
	expandPaths(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyEnv overlays SYMVIBE_* environment variables (highest precedence below CLI).
func applyEnv(c *Config) {
	if v := os.Getenv("SYMVIBE_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("SYMVIBE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Server.Port = n
		}
	}
	if v := os.Getenv("SYMVIBE_OPEN_BROWSER"); v != "" {
		c.Server.OpenBrowser = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("SYMVIBE_ACCESS"); v != "" {
		c.Server.Access = v
	}
	if v := os.Getenv("SYMVIBE_RUNNER_BACKEND"); v != "" {
		c.Runner.Backend = v
	}
	if v := os.Getenv("SYMVIBE_OPENCODE_BIN"); v != "" {
		c.Runner.OpencodeBin = v
	}
	if v := os.Getenv("SYMVIBE_WORKING_DIR"); v != "" {
		c.Runner.WorkingDir = v
	}
	if v := os.Getenv("SYMVIBE_RUNNER_MODE"); v != "" {
		c.Runner.Mode = v
	}
	if v := os.Getenv("SYMVIBE_SKIP_PERMISSIONS"); v != "" {
		c.Runner.SkipPermissions = v == "1" || strings.EqualFold(v, "true")
	}
}

func expandPaths(c *Config) {
	c.Runner.OpencodeBin = expandHome(c.Runner.OpencodeBin)
	c.Runner.WorkingDir = expandHome(c.Runner.WorkingDir)
}

// Validate checks structural integrity: every category model_ref resolves, the
// default category exists, the runner backend is known.
func (c *Config) Validate() error {
	switch c.Server.Access {
	case "", "loopback":
		if err := validateLoopbackHost(c.Server.Host); err != nil {
			return err
		}
	case "lan", "relay":
		if !c.Auth.Enabled {
			return fmt.Errorf("config: server.access %q requires auth.enabled = true", c.Server.Access)
		}
	default:
		return fmt.Errorf("config: unknown server.access %q (want loopback|lan|relay)", c.Server.Access)
	}
	switch c.Runner.Backend {
	case "opencode", "claudecode", "api":
	default:
		return fmt.Errorf("config: unknown runner.backend %q (want opencode|claudecode|api)", c.Runner.Backend)
	}
	for name, cat := range c.Categories {
		if cat.ModelRef == "" {
			return fmt.Errorf("config: category %q has no model_ref", name)
		}
		if _, ok := c.Models[cat.ModelRef]; !ok {
			return fmt.Errorf("config: category %q references unknown model %q", name, cat.ModelRef)
		}
	}
	if c.Defaults.Category == "" {
		return fmt.Errorf("config: defaults.category is empty")
	}
	if _, ok := c.Categories[c.Defaults.Category]; !ok {
		return fmt.Errorf("config: defaults.category %q is not a defined category", c.Defaults.Category)
	}
	return nil
}

func validateLoopbackHost(host string) error {
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("config: server.host %q must be a loopback IP address", host)
	}
	return nil
}

// ---------------------------------------------------------------------------
// XDG paths (ecosystem convention)
// ---------------------------------------------------------------------------

func configDir() string { return xdg("XDG_CONFIG_HOME", ".config") }

// DataDir returns the XDG data directory for symvibe (~/.local/share/symvibe).
func DataDir() string { return xdg("XDG_DATA_HOME", filepath.Join(".local", "share")) }
func dataDir() string { return DataDir() }
func cacheDir() string  { return xdg("XDG_CACHE_HOME", ".cache") }

func xdg(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return filepath.Join(v, "symvibe")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback, "symvibe")
}

// CyclesDir is where user-editable Baukasten files live.
func CyclesDir() string { return filepath.Join(dataDir(), "cycles") }

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}
