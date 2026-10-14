package config

import "testing"

// Every model the server calls has a default, so a bare .env never sends a
// request without a model (seen live: "model: Field required").
func TestModelDefaults(t *testing.T) {
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("ANTHROPIC_TRIAGE_MODEL", "")
	t.Setenv("ANTHROPIC_EFFORT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AnthropicModel == "" || cfg.AnthropicTriageModel == "" || cfg.AnthropicEffort == "" {
		t.Errorf("empty default: %+v", cfg)
	}
}

// Every account setting is read from the environment. Tests elsewhere build a
// Config by hand, so this is the one place the variable names are checked.
func TestAccountSettingsFromEnv(t *testing.T) {
	t.Setenv("RUNCOACH_OWNER_EMAIL", "leo@example.com")
	t.Setenv("RUNCOACH_OWNER_PASSWORD_HASH", "$2a$10$hash")
	t.Setenv("RUNCOACH_INVITE_CODE", "  run-with-leo ")
	t.Setenv("RUNCOACH_MEMBER_CAP", "7")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OwnerEmail != "leo@example.com" || cfg.OwnerPasswordHash != "$2a$10$hash" || cfg.InviteCode != "run-with-leo" || cfg.MemberCap != 7 {
		t.Errorf("config: %+v", cfg)
	}
	t.Setenv("RUNCOACH_MEMBER_CAP", "not a number")
	if cfg, _ = Load(); cfg.MemberCap != 10 {
		t.Errorf("bad cap falls back to 10, got %d", cfg.MemberCap)
	}
}
