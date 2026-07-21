package management

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestModelRoutesAPI_PutGetRoundtrip(t *testing.T) {
	h := newPromptRulesTestHandler(t, nil)
	body := map[string]any{
		"model-routes": []map[string]any{{
			"alias":            " auto ",
			"strategy":         "round-robin",
			"cooldown-seconds": 12,
			"models":           []string{" gpt-5.4 ", "claude-sonnet-4-6"},
		}},
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = newJSONReq(t, http.MethodPut, "/v0/management/model-routes", body)

	h.PutModelRoutes(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("PutModelRoutes status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(h.cfg.ModelRoutes) != 1 {
		t.Fatalf("model routes len = %d, want 1", len(h.cfg.ModelRoutes))
	}
	route := h.cfg.ModelRoutes[0]
	if route.Alias != "auto" || route.Strategy != config.ModelRouteStrategyRoundRobin || route.CooldownSeconds != 12 {
		t.Fatalf("route = %+v", route)
	}

	rec = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/model-routes", nil)
	h.GetModelRoutes(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetModelRoutes status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"alias":"auto"`) {
		t.Fatalf("GetModelRoutes body = %s", rec.Body.String())
	}
}

func TestModelRoutesAPI_PutRejectsNestedRouteTarget(t *testing.T) {
	h := newPromptRulesTestHandler(t, nil)
	body := []map[string]any{
		{"alias": "auto", "models": []string{"fast"}},
		{"alias": "fast", "models": []string{"gpt-5.4"}},
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = newJSONReq(t, http.MethodPut, "/v0/management/model-routes", body)

	h.PutModelRoutes(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PutModelRoutes status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(h.cfg.ModelRoutes) != 0 {
		t.Fatalf("model routes mutated on invalid input: %+v", h.cfg.ModelRoutes)
	}
}
