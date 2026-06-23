package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveOpencodeBin returns a usable opencode path using the same precedence as
// the rest of symvibe: the configured path (expanded) > PATH lookup >
// ~/.opencode/bin/opencode > "" (not found).
func ResolveOpencodeBin(configured string) string {
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

// SplitModel splits a canonical "provider/model" id into its provider and model
// parts. opencode `--model` and skill `--command` take the joined string; the
// serve-mode /session and /prompt_async bodies take the split object — keep this
// the single conversion point so both transports stay consistent.
func SplitModel(s string) (provider, model string) {
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}

// JoinModel is the inverse of SplitModel.
func JoinModel(provider, model string) string {
	if provider == "" {
		return model
	}
	return provider + "/" + model
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
