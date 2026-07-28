package engine

import (
	"fmt"
	"strings"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

// EvalRequiresReview evaluates a step's RequiresReview rule. It returns
// review=true with a reason when the step's attributes satisfy the predicate,
// meaning the engine must move the step to needs_review on completion instead
// of letting the cycle auto-advance. A nil rule or empty When never matches
// (fail-open: no review gate configured). An unparseable When also never
// matches; the reason explains why so a misconfigured rule is visible in the
// log rather than silently ignored.
//
// Supported When forms compare the step's category (the same attribute used
// for model bindings): "category == release", "category != release". Values
// may be single- or double-quoted.
func EvalRequiresReview(rule *config.RequiresReview, step *config.Step) (review bool, reason string) {
	if rule == nil || strings.TrimSpace(rule.When) == "" {
		return false, ""
	}
	when := strings.TrimSpace(rule.When)
	for _, op := range []string{"==", "!="} {
		lhs, rhs, found := strings.Cut(when, op)
		if !found {
			continue
		}
		attr := strings.TrimSpace(lhs)
		want := unquote(strings.TrimSpace(rhs))
		var got string
		switch attr {
		case "category":
			got = step.Category
		default:
			return false, fmt.Sprintf("unsupported attribute %q in %q — rule ignored", attr, rule.When)
		}
		matched := got == want
		if op == "!=" {
			matched = !matched
		}
		if matched {
			return true, fmt.Sprintf("%s %s %q satisfies %q", attr, op, want, rule.When)
		}
		return false, ""
	}
	return false, fmt.Sprintf("unparseable when %q — rule ignored", rule.When)
}

// unquote strips one layer of matching single or double quotes.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
