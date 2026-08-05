package runner

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "OpenAI key shape",
			input:    "Error using key sk-proj-1234567890abcdef12345678 in request",
			expected: "Error using key [REDACTED] in request",
		},
		{
			name:     "Anthropic key shape",
			input:    "API Key sk-ant-api03-abcdef1234567890-xyz invalid",
			expected: "API Key [REDACTED] invalid",
		},
		{
			name:     "Bearer token",
			input:    "Authorization: Bearer secret.jwt.token_here",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			name:     "api_key parameter",
			input:    "failed at https://api.example.com/v1?api_key=secret_12345&other=val",
			expected: "failed at https://api.example.com/v1?api_key=[REDACTED]&other=val",
		},
		{
			name:     "token query parameter",
			input:    "URL: https://host/socket?token=abcdef12345678",
			expected: "URL: https://host/socket?token=[REDACTED]",
		},
		{
			name:     "Clean string",
			input:    "Standard log output without sensitive tokens",
			expected: "Standard log output without sensitive tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeString(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeString(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeEvent_DefaultRedactsRaw(t *testing.T) {
	t.Setenv("SYMVIBE_RUNNER_DEBUG_RAW", "")
	t.Setenv("SYMVIBE_DEBUG_RAW", "")

	ev := RunEvent{
		Kind: EventLog,
		Text: "Executing tool with sk-1234567890",
		Err:  "Auth error Bearer token12345678",
		Raw:  `{"type":"log","text":"Executing tool with sk-1234567890"}`,
	}

	san := SanitizeEvent(ev)

	if san.Raw != "" {
		t.Errorf("expected Raw to be empty by default, got %q", san.Raw)
	}
	if san.Text != "Executing tool with [REDACTED]" {
		t.Errorf("unexpected Text: %q", san.Text)
	}
	if san.Err != "Auth error Bearer [REDACTED]" {
		t.Errorf("unexpected Err: %q", san.Err)
	}
}

func TestSanitizeEvent_DebugOptInKeepsRawSanitized(t *testing.T) {
	t.Setenv("SYMVIBE_RUNNER_DEBUG_RAW", "1")

	ev := RunEvent{
		Kind: EventLog,
		Text: "Connecting to https://host?token=secret12345678",
		Raw:  `{"token":"sk-1234567890"}`,
	}

	san := SanitizeEvent(ev)

	if san.Raw == "" {
		t.Errorf("expected Raw to be populated when debug opt-in is set")
	}
	if san.Raw != `{"token":"[REDACTED]"}` {
		t.Errorf("unexpected sanitized Raw: %q", san.Raw)
	}
}

func TestGuardrail_TraceSerialization(t *testing.T) {
	sensitiveLiterals := []string{
		"sk-proj-1234567890abcdef1234",
		"sk-ant-12345678901234567890",
		"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"api_key=supersecretkey12345",
		"?token=secretquerytoken12345",
	}

	for _, secret := range sensitiveLiterals {
		t.Run("secret_"+secret[:5], func(t *testing.T) {
			ev := SanitizeEvent(RunEvent{
				Kind: EventError,
				Text: "Encountered error with credential " + secret,
				Err:  "Failed auth " + secret,
				Raw:  `{"error":"` + secret + `"}`,
			})

			serialized, err := json.Marshal(ev)
			if err != nil {
				t.Fatalf("failed to marshal event: %v", err)
			}

			str := string(serialized)
			if stringContains(str, secret) {
				t.Errorf("sensitive literal %q survived trace serialization: %s", secret, str)
			}
		})
	}
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) > 0 && (hasSubstr(s, substr)))
}

func hasSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestSanitizeString_ProviderTokens covers the token families added in
// issue #140: GitHub (classic + fine-grained PATs), GitLab, AWS access
// keys, Google API keys, PEM private keys, and Bearer tokens whose
// charset includes base64/base64url characters (+ and /).
//
// Format-valid dummy tokens are assembled at runtime via concatenation
// so no secret-looking literal appears in source (GitHub secret
// scanning / push protection blocks such literals).
func TestSanitizeString_ProviderTokens(t *testing.T) {
	ghpToken := "ghp_" + strings.Repeat("A", 36)
	ghPat := "github_" + "pat_" + strings.Repeat("A", 30)
	glpatToken := "glpat" + "-" + strings.Repeat("A", 20)
	awsAccessKey := "AKIA" + strings.Repeat("A", 16)
	awsSessionKey := "ASIA" + strings.Repeat("B", 16)
	googleAPIKey := "AIza" + strings.Repeat("A", 35)
	pemHeader := "-----" + "BEGIN RSA PRIVATE KEY-----"
	pemBlock := pemHeader + "\nMIIEpQIBAAKCAQEA7d3hXq\n" + "-----" + "END RSA PRIVATE KEY-----"
	jwtToken := "eyJhbGciOiJIUzI1NiJ9" + ".eyJzdWIiOiIxMjM0NTY3ODkwIn0" + ".ab+cde/fghiJKLMNOPQRSTUVWXYZ0123456789"

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"GitHub classic PAT", "clone failed: token " + ghpToken + " is invalid", "clone failed: token [REDACTED] is invalid"},
		{"GitHub fine-grained PAT", "auth with " + ghPat + " expired", "auth with [REDACTED] expired"},
		{"GitLab PAT", "gitlab rejected " + glpatToken + " for user x", "gitlab rejected [REDACTED] for user x"},
		{"AWS access key", "config uses " + awsAccessKey + " in region us-east-1", "config uses [REDACTED] in region us-east-1"},
		{"AWS session key", "sts returned " + awsSessionKey + " for role deploy", "sts returned [REDACTED] for role deploy"},
		{"Google API key", "maps call failed with " + googleAPIKey + " (quota)", "maps call failed with [REDACTED] (quota)"},
		{"PEM header only", "cannot parse " + pemHeader + " in file", "cannot parse [REDACTED] in file"},
		{"PEM full block", "key material: " + pemBlock + " (end)", "key material: [REDACTED] (end)"},
		{"Bearer JWT with base64url + and /", "Authorization: Bearer " + jwtToken, "Authorization: Bearer [REDACTED]"},
		{"Bearer base64 with + and /", "Authorization: Bearer abc+/def=ghi", "Authorization: Bearer [REDACTED]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeString(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeString(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}
