package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// MinOpencodeVersion is the oldest opencode release symvibe supports. Older
// binaries may lack `opencode run --format json` or the event shape the runner
// expects.
const MinOpencodeVersion = "1.17.0"

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

// OpencodeVersion runs `opencode --version` and returns the trimmed output.
// It returns "",false when the binary is missing or the call fails.
func OpencodeVersion(bin string) (string, bool) {
	if bin == "" {
		return "", false
	}
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// CheckOpencodeVersion reports whether the installed opencode meets the minimum
// version. It accepts free-form version strings like "1.17.9" or
// "opencode version 1.17.9" and extracts the first semver-looking token.
func CheckOpencodeVersion(ver string) (ok bool, detail string) {
	parsed := extractVersion(ver)
	if parsed == "" {
		return false, fmt.Sprintf("could not parse version %q", ver)
	}
	cmp := compareVersions(parsed, MinOpencodeVersion)
	switch {
	case cmp < 0:
		return false, fmt.Sprintf("version %s is older than required %s; upgrade opencode", parsed, MinOpencodeVersion)
	default:
		return true, fmt.Sprintf("version %s", parsed)
	}
}

// extractVersion pulls the first "X.Y.Z" token from a string, stripping an
// optional leading "v".
func extractVersion(s string) string {
	for _, f := range strings.Fields(s) {
		f = strings.TrimPrefix(f, "v")
		if isVersion(f) {
			return f
		}
	}
	return ""
}

func isVersion(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			// allow a trailing non-numeric pre-release token like "1.17.9-beta"
			if strings.Contains(p, "-") {
				pre := strings.SplitN(p, "-", 2)[0]
				if _, err := strconv.Atoi(pre); err == nil {
					continue
				}
			}
			return false
		}
	}
	return true
}

// compareVersions compares two "X.Y.Z" strings. It returns -1/0/1. Shorter
// versions are padded with zeros so "1.17" equals "1.17.0".
func compareVersions(a, b string) int {
	an := normalizeVersion(a)
	bn := normalizeVersion(b)
	for len(an) < len(bn) {
		an = append(an, 0)
	}
	for len(bn) < len(an) {
		bn = append(bn, 0)
	}
	for i := 0; i < len(an); i++ {
		if an[i] < bn[i] {
			return -1
		}
		if an[i] > bn[i] {
			return 1
		}
	}
	return 0
}

func normalizeVersion(s string) []int {
	parts := strings.Split(s, "-")[0]
	out := []int{}
	for _, p := range strings.Split(parts, ".") {
		if n, err := strconv.Atoi(p); err == nil {
			out = append(out, n)
		}
	}
	return out
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
		return filepath.Join(home, strings.TrimPrefix(p, "~/"))
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
