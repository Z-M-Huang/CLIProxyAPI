package config

import (
	"fmt"
	"strings"
)

const (
	ModelRouteStrategyPriority   = "priority"
	ModelRouteStrategyRoundRobin = "round-robin"

	DefaultModelRouteCooldownSeconds = 60
)

// ModelRoute maps a client-visible model alias to concrete upstream models.
type ModelRoute struct {
	Alias           string   `yaml:"alias" json:"alias"`
	Strategy        string   `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	CooldownSeconds int      `yaml:"cooldown-seconds,omitempty" json:"cooldown-seconds,omitempty"`
	Models          []string `yaml:"models" json:"models"`
}

// NormalizeModelRoutes trims model route fields and applies defaults.
func (cfg *Config) NormalizeModelRoutes() {
	if cfg == nil || len(cfg.ModelRoutes) == 0 {
		return
	}
	routes := make([]ModelRoute, 0, len(cfg.ModelRoutes))
	for _, route := range cfg.ModelRoutes {
		route.Alias = strings.TrimSpace(route.Alias)
		route.Strategy = strings.ToLower(strings.TrimSpace(route.Strategy))
		if route.Strategy == "" {
			route.Strategy = ModelRouteStrategyPriority
		}
		if route.CooldownSeconds <= 0 {
			route.CooldownSeconds = DefaultModelRouteCooldownSeconds
		}
		models := make([]string, 0, len(route.Models))
		for _, model := range route.Models {
			model = strings.TrimSpace(model)
			if model != "" {
				models = append(models, model)
			}
		}
		route.Models = models
		routes = append(routes, route)
	}
	cfg.ModelRoutes = routes
}

// ValidateModelRoutes returns the first configured model route validation error.
func (cfg *Config) ValidateModelRoutes() error {
	if cfg == nil || len(cfg.ModelRoutes) == 0 {
		return nil
	}
	aliases := make(map[string]int, len(cfg.ModelRoutes))
	for i, route := range cfg.ModelRoutes {
		alias := strings.TrimSpace(route.Alias)
		if alias == "" {
			return fmt.Errorf("model-routes[%d]: alias is required", i)
		}
		if hasModelSuffix(alias) {
			return fmt.Errorf("model-routes[%d] %q: alias must not include a thinking suffix", i, alias)
		}
		aliasKey := strings.ToLower(alias)
		if previous, exists := aliases[aliasKey]; exists {
			return fmt.Errorf("model-routes[%d] %q: duplicate alias (also at index %d)", i, alias, previous)
		}
		aliases[aliasKey] = i
		switch strings.ToLower(strings.TrimSpace(route.Strategy)) {
		case "", ModelRouteStrategyPriority, ModelRouteStrategyRoundRobin:
		default:
			return fmt.Errorf("model-routes[%d] %q: strategy must be %q or %q", i, alias, ModelRouteStrategyPriority, ModelRouteStrategyRoundRobin)
		}
		if route.CooldownSeconds < 0 {
			return fmt.Errorf("model-routes[%d] %q: cooldown-seconds must be >= 0", i, alias)
		}
		if len(route.Models) == 0 {
			return fmt.Errorf("model-routes[%d] %q: at least one model is required", i, alias)
		}
		seenModels := make(map[string]int, len(route.Models))
		for j, model := range route.Models {
			model = strings.TrimSpace(model)
			if model == "" {
				return fmt.Errorf("model-routes[%d] %q models[%d]: model is required", i, alias, j)
			}
			modelKey := strings.ToLower(model)
			if previous, exists := seenModels[modelKey]; exists {
				return fmt.Errorf("model-routes[%d] %q models[%d] %q: duplicate model (also at index %d)", i, alias, j, model, previous)
			}
			seenModels[modelKey] = j
		}
	}
	for i, route := range cfg.ModelRoutes {
		alias := strings.TrimSpace(route.Alias)
		for j, model := range route.Models {
			baseModel := strings.TrimSpace(modelBaseName(model))
			if baseModel == "" {
				continue
			}
			if targetIndex, exists := aliases[strings.ToLower(baseModel)]; exists {
				return fmt.Errorf("model-routes[%d] %q models[%d] %q: route target must not reference route alias at index %d", i, alias, j, model, targetIndex)
			}
		}
	}
	return nil
}

func hasModelSuffix(model string) bool {
	model = strings.TrimSpace(model)
	open := strings.LastIndex(model, "(")
	return open > 0 && strings.HasSuffix(model, ")")
}

func modelBaseName(model string) string {
	model = strings.TrimSpace(model)
	open := strings.LastIndex(model, "(")
	if open <= 0 || !strings.HasSuffix(model, ")") {
		return model
	}
	return strings.TrimSpace(model[:open])
}
