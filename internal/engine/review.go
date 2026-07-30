// Package engine — review-context provider for review steps and recipe results.
package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

// ReviewContextSchemaVersion is the current schema for ReviewContext.
const ReviewContextSchemaVersion = 1

// ReviewContextProvider supplies structured review metadata about a step's
// changes and risk, used by review steps to scope their analysis.
type ReviewContextProvider interface {
	// Name returns the human-readable provider identifier.
	Name() string

	// Provide returns a compact review context for the given step. The dir
	// is the working directory (repo root) for git operations.
	// Returns nil context and nil error when the provider is absent or
	// unavailable (graceful degradation).
	Provide(ctx context.Context, dir string, stepID string) (*ReviewContext, error)
}

// ReviewContext holds structured metadata about a review step's scope.
type ReviewContext struct {
	SchemaVersion int    `json:"schema_version"`
	Provider      string `json:"provider"`
	GeneratedAt   int64  `json:"generated_at"` // unix millis

	// ChangedFiles lists files modified since the last stable base.
	ChangedFiles []string `json:"changed_files,omitempty"`
	// ImpactedFiles lists files that may need updates due to the changes.
	ImpactedFiles []string `json:"impacted_files,omitempty"`

	// Risk is a coarse assessment ("low", "medium", "high", "unknown").
	Risk string `json:"risk,omitempty"`
	// TestGaps lists areas where tests are missing or incomplete.
	TestGaps []string `json:"test_gaps,omitempty"`

	// Stale indicates the context could not be fully computed (timeout,
	// provider unavailable, etc.) and should be treated as advisory.
	Stale bool `json:"stale,omitempty"`
}

// ---------------------------------------------------------------------------
// NoopReviewProvider — default fallback, always returns nil (no context).
// ---------------------------------------------------------------------------

type NoopReviewProvider struct{}

func (NoopReviewProvider) Name() string { return "none" }

func (NoopReviewProvider) Provide(_ context.Context, _ string, _ string) (*ReviewContext, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// GitReviewProvider — simple provider that reads git diff for context.
// ---------------------------------------------------------------------------

// GitReviewProvider generates review context from the local git repository.
type GitReviewProvider struct {
	Timeout time.Duration
}

func NewGitReviewProvider() *GitReviewProvider {
	return &GitReviewProvider{Timeout: 5 * time.Second}
}

func (p *GitReviewProvider) Name() string { return "git" }

func (p *GitReviewProvider) Provide(ctx context.Context, dir, stepID string) (*ReviewContext, error) {
	if dir == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	changed, err := gitChangedFiles(ctx, dir)
	if err != nil {
		return &ReviewContext{
			SchemaVersion: ReviewContextSchemaVersion,
			Provider:      "git",
			GeneratedAt:   nowMs(),
			Risk:          "unknown",
			Stale:         true,
		}, nil // degrade gracefully
	}

	risk := assessRisk(changed)
	return &ReviewContext{
		SchemaVersion: ReviewContextSchemaVersion,
		Provider:      "git",
		GeneratedAt:   nowMs(),
		ChangedFiles:  changed,
		Risk:          risk,
	}, nil
}

// gitChangedFiles returns files modified, added, or deleted vs HEAD.
func gitChangedFiles(ctx context.Context, dir string) ([]string, error) {
	out, err := runIn(ctx, dir, "git", "diff", "--name-only", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	lines := splitLines(string(out))
	if len(lines) == 0 {
		// Also check for staged but uncommitted changes.
		out2, err2 := runIn(ctx, dir, "git", "diff", "--name-only", "--cached")
		if err2 != nil {
			return lines, nil
		}
		lines = append(lines, splitLines(string(out2))...)
	}
	return unique(lines), nil
}

// assessRisk assigns a coarse risk level based on changed file patterns.
func assessRisk(files []string) string {
	if len(files) == 0 {
		return "low"
	}
	hasChanges := false
	hasMedium := false
	for _, f := range files {
		hasChanges = true
		if containsGlob(f, "go.mod") || containsGlob(f, "go.sum") || containsGlob(f, "Package.swift") || containsGlob(f, "project.yml") {
			return "high"
		}
		if containsGlob(f, "internal/engine/*") || containsGlob(f, "internal/runner/*") {
			hasMedium = true
		}
		if containsGlob(f, "*_test.go") || containsGlob(f, "*.swift") || containsGlob(f, "client/*") {
			hasMedium = true
		}
	}
	if !hasChanges {
		return "low"
	}
	if hasMedium {
		return "medium"
	}
	return "low"
}

// containsGlob reports whether path matches the glob pattern (simple * only).
func containsGlob(path, pattern string) bool {
	return matchGlob(path, pattern)
}

func matchGlob(path, pattern string) bool {
	// Simple glob: * matches any sequence of non-separator characters.
	// Just check for suffix/prefix/infix matches.
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[0] == '*' {
		return len(path) >= len(pattern)-1 && path[len(path)-len(pattern)+1:] == pattern[1:]
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		return len(path) >= len(pattern)-1 && path[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	return path == pattern
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				lines = append(lines, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// EvalRequiresReview evaluates a step's declarative risk rule. When the rule
// matches the step's attributes (e.g. "category == release") the step should
// be moved to needs_review instead of done, halting the autonomous walk for a
// human ack. Returns (false, "") when the rule is nil or does not match.
// This function was missing from the codebase after PR #113 and is defined
// here to resolve the pre-existing undefined reference.
func EvalRequiresReview(rule *config.RequiresReview, step *config.Step) (match bool, reason string) {
	if rule == nil || rule.When == "" {
		return false, ""
	}
	// Supported forms: "category == <value>", "category != <value>"
	parts := strings.Fields(rule.When)
	if len(parts) != 3 {
		return false, fmt.Sprintf("unparseable rule %q", rule.When)
	}
	attr, op, val := parts[0], parts[1], parts[2]
	val = strings.Trim(val, `"'`)
	switch attr {
	case "category":
		switch op {
		case "==":
			if step.Category == val {
				return true, fmt.Sprintf("category %q matches %q", step.Category, rule.When)
			}
		case "!=":
			if step.Category != val {
				return true, fmt.Sprintf("category %q != %q", step.Category, val)
			}
		}
	}
	return false, ""
}

func unique(items []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}
