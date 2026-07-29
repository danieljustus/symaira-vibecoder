package runner

import (
	"os"
	"regexp"
	"strings"
)

var credPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsk-[a-zA-Z0-9_\-]{8,}`),
	regexp.MustCompile(`(?i)\bBearer\s+[a-zA-Z0-9_\-\.=]+`),
	regexp.MustCompile(`(?i)\b(api_key|apikey)=[^&\s]+`),
	regexp.MustCompile(`(?i)[?&]token=[^&\s]+`),
}

func isDebugRawEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SYMVIBE_RUNNER_DEBUG_RAW")))
	if v == "1" || v == "true" || v == "yes" {
		return true
	}
	v2 := strings.ToLower(strings.TrimSpace(os.Getenv("SYMVIBE_DEBUG_RAW")))
	return v2 == "1" || v2 == "true" || v2 == "yes"
}

// SanitizeString scrubs credential patterns from text or error strings.
func SanitizeString(s string) string {
	if s == "" {
		return ""
	}
	res := s
	for _, re := range credPatterns {
		pattern := re.String()
		if strings.Contains(pattern, "token=") {
			res = re.ReplaceAllStringFunc(res, func(match string) string {
				if strings.HasPrefix(match, "?") {
					return "?token=[REDACTED]"
				}
				return "&token=[REDACTED]"
			})
		} else if strings.Contains(pattern, "api_key") || strings.Contains(pattern, "apikey") {
			res = re.ReplaceAllStringFunc(res, func(match string) string {
				idx := strings.Index(match, "=")
				if idx != -1 {
					return match[:idx+1] + "[REDACTED]"
				}
				return match
			})
		} else if strings.Contains(pattern, "Bearer") {
			res = re.ReplaceAllString(res, "Bearer [REDACTED]")
		} else {
			res = re.ReplaceAllString(res, "[REDACTED]")
		}
	}
	return res
}

// SanitizeEvent applies raw redaction and text/error scrubbing to a RunEvent.
func SanitizeEvent(ev RunEvent) RunEvent {
	if !isDebugRawEnabled() {
		ev.Raw = ""
	} else if ev.Raw != "" {
		ev.Raw = SanitizeString(ev.Raw)
	}
	ev.Text = SanitizeString(ev.Text)
	ev.Err = SanitizeString(ev.Err)
	return ev
}
