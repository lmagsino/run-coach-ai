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
