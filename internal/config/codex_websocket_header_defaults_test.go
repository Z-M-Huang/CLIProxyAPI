package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOptional_CodexHeaderDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configYAML := []byte(`
codex-header-defaults:
  user-agent: "  my-codex-client/1.0  "
  beta-features: "  feature-a,feature-b  "
`)
	if err := os.WriteFile(configPath, configYAML, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if got := cfg.CodexHeaderDefaults.UserAgent; got != "my-codex-client/1.0" {
		t.Fatalf("UserAgent = %q, want %q", got, "my-codex-client/1.0")
	}
	if got := cfg.CodexHeaderDefaults.BetaFeatures; got != "feature-a,feature-b" {
		t.Fatalf("BetaFeatures = %q, want %q", got, "feature-a,feature-b")
	}
}

func TestLoadConfigOptional_ProviderUserAgentHeaderDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configYAML := []byte(`
gemini-cli-header-defaults:
  user-agent: "  custom-gemini-cli/1.0  "
openai-compatibility-header-defaults:
  user-agent: "  custom-openai-compat/1.0  "
kimi-header-defaults:
  user-agent: "  custom-kimi/1.0  "
antigravity-header-defaults:
  user-agent: "  antigravity/1.23.2 linux/x64  "
`)
	if err := os.WriteFile(configPath, configYAML, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if got := cfg.GeminiCLIHeaderDefaults.UserAgent; got != "custom-gemini-cli/1.0" {
		t.Fatalf("GeminiCLIHeaderDefaults.UserAgent = %q, want %q", got, "custom-gemini-cli/1.0")
	}
	if got := cfg.OpenAICompatibilityHeaderDefaults.UserAgent; got != "custom-openai-compat/1.0" {
		t.Fatalf("OpenAICompatibilityHeaderDefaults.UserAgent = %q, want %q", got, "custom-openai-compat/1.0")
	}
	if got := cfg.KimiHeaderDefaults.UserAgent; got != "custom-kimi/1.0" {
		t.Fatalf("KimiHeaderDefaults.UserAgent = %q, want %q", got, "custom-kimi/1.0")
	}
	if got := cfg.AntigravityHeaderDefaults.UserAgent; got != "antigravity/1.23.2 linux/x64" {
		t.Fatalf("AntigravityHeaderDefaults.UserAgent = %q, want %q", got, "antigravity/1.23.2 linux/x64")
	}

	parsed, err := ParseConfigBytes(configYAML)
	if err != nil {
		t.Fatalf("ParseConfigBytes() error = %v", err)
	}
	if got := parsed.GeminiCLIHeaderDefaults.UserAgent; got != "custom-gemini-cli/1.0" {
		t.Fatalf("ParseConfigBytes GeminiCLIHeaderDefaults.UserAgent = %q, want %q", got, "custom-gemini-cli/1.0")
	}
	if got := parsed.OpenAICompatibilityHeaderDefaults.UserAgent; got != "custom-openai-compat/1.0" {
		t.Fatalf("ParseConfigBytes OpenAICompatibilityHeaderDefaults.UserAgent = %q, want %q", got, "custom-openai-compat/1.0")
	}
	if got := parsed.KimiHeaderDefaults.UserAgent; got != "custom-kimi/1.0" {
		t.Fatalf("ParseConfigBytes KimiHeaderDefaults.UserAgent = %q, want %q", got, "custom-kimi/1.0")
	}
	if got := parsed.AntigravityHeaderDefaults.UserAgent; got != "antigravity/1.23.2 linux/x64" {
		t.Fatalf("ParseConfigBytes AntigravityHeaderDefaults.UserAgent = %q, want %q", got, "antigravity/1.23.2 linux/x64")
	}
}

func TestLoadConfigOptional_CodexIdentityConfuse(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configYAML := []byte(`
codex:
  identity-confuse: true
`)
	if err := os.WriteFile(configPath, configYAML, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if !cfg.Codex.IdentityConfuse {
		t.Fatalf("IdentityConfuse = false, want true")
	}
}
