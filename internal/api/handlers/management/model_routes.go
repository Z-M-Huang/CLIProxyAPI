package management

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

// GetModelRoutes returns the configured logical model route aliases.
func (h *Handler) GetModelRoutes(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := append([]config.ModelRoute(nil), h.cfg.ModelRoutes...)
	c.JSON(http.StatusOK, gin.H{"model-routes": out})
}

// PutModelRoutes replaces the full model-routes list.
func (h *Handler) PutModelRoutes(c *gin.Context) {
	routes, ok := readModelRoutesBody(c)
	if !ok {
		return
	}
	candidate := config.Config{}
	candidate.ModelRoutes = append([]config.ModelRoute(nil), routes...)
	candidate.NormalizeModelRoutes()
	if err := candidate.ValidateModelRoutes(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_model_routes", "message": err.Error()})
		return
	}
	h.mu.Lock()
	clone := h.cfg.CloneForRuntime()
	clone.ModelRoutes = append([]config.ModelRoute(nil), candidate.ModelRoutes...)
	previous := h.cfg
	h.cfg = clone
	snapshot, saved := h.saveConfigAndSnapshotLocked(c)
	if !saved {
		h.cfg = previous
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.reloadConfigAfterManagementSave(c.Request.Context(), snapshot)
	c.JSON(http.StatusOK, gin.H{"ok": true, "model-routes": clone.ModelRoutes})
}

func readModelRoutesBody(c *gin.Context) ([]config.ModelRoute, bool) {
	if c.Request == nil || c.Request.Body == nil {
		return nil, true
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json", "message": err.Error()})
		return nil, false
	}
	if strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return nil, true
	}
	var routes []config.ModelRoute
	if err := json.Unmarshal(raw, &routes); err == nil {
		return routes, true
	}
	var wrapped struct {
		ModelRoutes []config.ModelRoute `json:"model-routes"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json", "message": fmt.Sprintf("expected array or object with model-routes: %v", err)})
		return nil, false
	}
	return wrapped.ModelRoutes, true
}
