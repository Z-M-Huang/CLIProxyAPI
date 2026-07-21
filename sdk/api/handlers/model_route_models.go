package handlers

import (
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

// AddModelRouteAliases appends configured route aliases to a protocol's model catalog.
func (h *BaseAPIHandler) AddModelRouteAliases(models []map[string]any, protocol string) []map[string]any {
	if h == nil || h.Cfg == nil || len(h.Cfg.ModelRoutes) == 0 {
		return models
	}
	seen := make(map[string]struct{}, len(models)+len(h.Cfg.ModelRoutes))
	for _, model := range models {
		for _, key := range []string{"id", "name"} {
			if value, ok := model[key].(string); ok {
				normalized := strings.TrimPrefix(strings.TrimSpace(value), "models/")
				if normalized != "" {
					seen[strings.ToLower(normalized)] = struct{}{}
				}
			}
		}
	}
	out := append([]map[string]any(nil), models...)
	for _, route := range configuredModelRoutes(h.Cfg) {
		alias := strings.TrimSpace(route.Alias)
		if alias == "" {
			continue
		}
		key := strings.ToLower(alias)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, modelRouteAliasEntry(alias, protocol, route.Strategy))
	}
	return out
}

func modelRouteAliasEntry(alias, protocol, strategy string) map[string]any {
	entry := map[string]any{
		"id":           alias,
		"object":       "model",
		"created":      int64(0),
		"owned_by":     "cli-proxy-api",
		"display_name": alias,
		"description":  "CLI Proxy model route",
		"route": map[string]any{
			"strategy": firstNonEmptyString(strategy, config.ModelRouteStrategyPriority),
		},
	}
	if strings.EqualFold(protocol, "gemini") {
		entry["name"] = alias
		entry["displayName"] = alias
		entry["description"] = "CLI Proxy model route"
		entry["supportedGenerationMethods"] = []string{"generateContent", "countTokens"}
	}
	return entry
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
