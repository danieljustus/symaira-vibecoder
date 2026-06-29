package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/danieljustus/symaira-vibecoder/internal/config"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestVersionCommand(t *testing.T) {
	cmd := versionCmd()
	cmd.SetArgs([]string{})

	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("version command error: %v", err)
		}
	})

	if out == "" {
		t.Fatal("version command produced no output")
	}
	if !contains(out, "symvibe") {
		t.Fatalf("expected output to contain 'symvibe', got: %s", out)
	}
}

func TestDoctorCommand(t *testing.T) {
	cmd := doctorCmd()
	cmd.SetArgs([]string{})

	out := captureStdout(t, func() {
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("doctor command error: %v", err)
		}
	})

	if out == "" {
		t.Fatal("doctor command produced no output")
	}
	if !contains(out, "symvibe doctor") {
		t.Fatalf("expected output to contain 'symvibe doctor', got: %s", out)
	}
}

func TestDoctorCommandJSON(t *testing.T) {
	cmd := doctorCmd()
	cmd.SetArgs([]string{"--json"})

	out := captureStdout(t, func() {
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("doctor --json error: %v", err)
		}
	})

	if out == "" {
		t.Fatal("doctor --json produced no output")
	}
	if !contains(out, "opencode_ok") {
		t.Fatalf("expected JSON output to contain 'opencode_ok', got: %s", out)
	}
}

func TestDeriveBindHost(t *testing.T) {
	tests := []struct {
		name   string
		access string
		host   string
		want   string
	}{
		{"default loopback", "", "", "127.0.0.1"},
		{"explicit loopback", "loopback", "127.0.0.1", "127.0.0.1"},
		{"lan with default host", "lan", "127.0.0.1", "0.0.0.0"},
		{"lan with empty host", "lan", "", "0.0.0.0"},
		{"lan with custom host", "lan", "192.168.1.100", "192.168.1.100"},
		{"relay", "relay", "127.0.0.1", "0.0.0.0"},
		{"custom host no access", "", "10.0.0.1", "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Server.Access = tt.access
			cfg.Server.Host = tt.host
			got := deriveBindHost(cfg)
			if got != tt.want {
				t.Fatalf("deriveBindHost(%q, %q) = %q, want %q", tt.access, tt.host, got, tt.want)
			}
		})
	}
}

func TestGeneratePairCode(t *testing.T) {
	code := generatePairCode()
	if len(code) != 6 {
		t.Fatalf("expected 6-character code, got %d: %s", len(code), code)
	}
	for _, c := range code {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'H') || c == 'J' || c == 'K' || (c >= 'L' && c <= 'N') || (c >= 'P' && c <= 'Z')) {
			t.Fatalf("unexpected character in pair code: %c", c)
		}
	}
}

func TestBuildPairPayload(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Host = "192.168.1.100"
	cfg.Server.Port = 4317

	payload := buildPairPayload(cfg, "abc123", "XYZ789")
	if payload == "" {
		t.Fatal("buildPairPayload returned empty string")
	}
	if !contains(payload, "symvibe://pair") {
		t.Fatalf("expected payload to contain 'symvibe://pair', got: %s", payload)
	}
	if !contains(payload, "h=192.168.1.100") {
		t.Fatalf("expected payload to contain host, got: %s", payload)
	}
	if !contains(payload, "p=4317") {
		t.Fatalf("expected payload to contain port, got: %s", payload)
	}
}

func TestBuildPairPayloadWildcardHost(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 4317

	payload := buildPairPayload(cfg, "abc123", "XYZ789")
	if !contains(payload, "h=127.0.0.1") {
		t.Fatalf("expected wildcard host to be replaced with 127.0.0.1, got: %s", payload)
	}
}

func TestServeFlags(t *testing.T) {
	cmd := serveCmd()

	if cmd.Use != "serve" {
		t.Fatalf("expected Use='serve', got %q", cmd.Use)
	}

	flags := cmd.Flags()
	if flags.Lookup("host") == nil {
		t.Fatal("serve command missing --host flag")
	}
	if flags.Lookup("port") == nil {
		t.Fatal("serve command missing --port flag")
	}
	if flags.Lookup("dir") == nil {
		t.Fatal("serve command missing --dir flag")
	}
	if flags.Lookup("no-open") == nil {
		t.Fatal("serve command missing --no-open flag")
	}
	if flags.Lookup("access") == nil {
		t.Fatal("serve command missing --access flag")
	}
}

func TestRootCommand(t *testing.T) {
	root := &cobra.Command{
		Use:   "symvibe",
		Short: "test root",
	}
	root.AddCommand(serveCmd(), doctorCmd(), versionCmd(), pairCmd())

	commands := make(map[string]bool)
	for _, cmd := range root.Commands() {
		commands[cmd.Use] = true
	}

	for _, want := range []string{"serve", "doctor", "version", "pair"} {
		if !commands[want] {
			t.Fatalf("root command missing subcommand %q", want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
