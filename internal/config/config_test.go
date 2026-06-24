package config

import "testing"

func TestValidateAccessDefaultLoopback(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}

func TestValidateAccessLoopbackRequiresLoopbackHost(t *testing.T) {
	cfg := Default()
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Access = "loopback"
	if err := cfg.Validate(); err == nil {
		t.Fatal("loopback access with non-loopback host must fail")
	}
}

func TestValidateAccessLanRequiresAuth(t *testing.T) {
	cfg := Default()
	cfg.Server.Access = "lan"
	cfg.Auth.Enabled = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("lan access without auth must fail")
	}
}

func TestValidateAccessLanWithAuth(t *testing.T) {
	cfg := Default()
	cfg.Server.Access = "lan"
	cfg.Auth.Enabled = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("lan with auth should validate: %v", err)
	}
}

func TestValidateAccessRelayRequiresAuth(t *testing.T) {
	cfg := Default()
	cfg.Server.Access = "relay"
	cfg.Auth.Enabled = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("relay access without auth must fail")
	}
}

func TestValidateAccessRelayWithAuth(t *testing.T) {
	cfg := Default()
	cfg.Server.Access = "relay"
	cfg.Auth.Enabled = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("relay with auth should validate: %v", err)
	}
}

func TestValidateAccessUnknown(t *testing.T) {
	cfg := Default()
	cfg.Server.Access = "public"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unknown access mode must fail")
	}
}

func TestValidateEmptyAccessDefaultsToLoopback(t *testing.T) {
	cfg := Default()
	cfg.Server.Access = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("empty access should be treated as loopback: %v", err)
	}
}
