package config

import (
	"strings"
	"testing"
)

func TestParseConfigBytesModelRoutes_NormalizesDefaults(t *testing.T) {
	cfg, err := ParseConfigBytes([]byte(`
model-routes:
  - alias: " auto "
    models:
      - " claude-sonnet-4-6 "
      - "gpt-5.4"
`))
	if err != nil {
		t.Fatalf("ParseConfigBytes() error = %v", err)
	}
	if len(cfg.ModelRoutes) != 1 {
		t.Fatalf("model routes len = %d, want 1", len(cfg.ModelRoutes))
	}
	route := cfg.ModelRoutes[0]
	if route.Alias != "auto" {
		t.Fatalf("alias = %q, want auto", route.Alias)
	}
	if route.Strategy != ModelRouteStrategyPriority {
		t.Fatalf("strategy = %q, want %q", route.Strategy, ModelRouteStrategyPriority)
	}
	if route.CooldownSeconds != DefaultModelRouteCooldownSeconds {
		t.Fatalf("cooldown = %d, want %d", route.CooldownSeconds, DefaultModelRouteCooldownSeconds)
	}
	if got := strings.Join(route.Models, ","); got != "claude-sonnet-4-6,gpt-5.4" {
		t.Fatalf("models = %q", got)
	}
}

func TestValidateModelRoutesRejectsNestedRouteTarget(t *testing.T) {
	cfg := &Config{SDKConfig: SDKConfig{ModelRoutes: []ModelRoute{
		{Alias: "auto", Models: []string{"fast"}},
		{Alias: "fast", Models: []string{"gpt-5.4"}},
	}}}
	cfg.NormalizeModelRoutes()
	err := cfg.ValidateModelRoutes()
	if err == nil {
		t.Fatal("ValidateModelRoutes() error = nil, want nested route target error")
	}
	if !strings.Contains(err.Error(), "must not reference route alias") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateModelRoutesRejectsDuplicateAlias(t *testing.T) {
	cfg := &Config{SDKConfig: SDKConfig{ModelRoutes: []ModelRoute{
		{Alias: "Auto", Models: []string{"gpt-5.4"}},
		{Alias: "auto", Models: []string{"claude-sonnet-4-6"}},
	}}}
	cfg.NormalizeModelRoutes()
	err := cfg.ValidateModelRoutes()
	if err == nil {
		t.Fatal("ValidateModelRoutes() error = nil, want duplicate alias error")
	}
	if !strings.Contains(err.Error(), "duplicate alias") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateModelRoutesRejectsAliasSuffix(t *testing.T) {
	cfg := &Config{SDKConfig: SDKConfig{ModelRoutes: []ModelRoute{{Alias: "auto(high)", Models: []string{"gpt-5.4"}}}}}
	cfg.NormalizeModelRoutes()
	err := cfg.ValidateModelRoutes()
	if err == nil {
		t.Fatal("ValidateModelRoutes() error = nil, want suffix alias error")
	}
	if !strings.Contains(err.Error(), "must not include a thinking suffix") {
		t.Fatalf("error = %v", err)
	}
}
