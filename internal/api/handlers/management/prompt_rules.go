package management

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/executor/helps"
)

// GetPromptRules returns the current configured prompt rules.
func (h *Handler) GetPromptRules(c *gin.Context) {
	out := make([]config.PromptRule, 0)
	if cfg := h.cfg(); cfg != nil {
		out = append(out, cfg.PromptRules...)
	}
	if out == nil {
		out = []config.PromptRule{}
	}
	c.JSON(http.StatusOK, gin.H{"prompt-rules": out})
}

// PutPromptRules replaces the entire prompt-rules list. Validation is strict —
// any invalid rule causes a 400 with the offending rule's name and reason.
// This is the primary write path used by the management UI.
func (h *Handler) PutPromptRules(c *gin.Context) {
	rules, ok := readPromptRulesBody(c)
	if !ok {
		return
	}
	candidate := config.Config{PromptRules: rules}
	candidate.NormalizePromptRules()
	if err := candidate.ValidatePromptRules(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	cur := make([]config.PromptRule, 0)
	if cfg := h.cfg(); cfg != nil {
		cur = append(cur, cfg.PromptRules...)
	}
	h.applyPromptRulesAndPersist(c, candidate.PromptRules)
}

// PatchPromptRule upserts a single rule. The body shape mirrors other list
// PATCH endpoints in this package:
//
//	{"index": <int>?, "match": "<name>"?, "value": <PromptRule>}
//
// If both index and match are absent (or no match found), the rule is appended.
func (h *Handler) PatchPromptRule(c *gin.Context) {
	var body struct {
		Index *int               `json:"index"`
		Match *string            `json:"match"`
		Value *config.PromptRule `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	cur := make([]config.PromptRule, 0)
	if cfg := h.cfg(); cfg != nil {
		cur = append(cur, cfg.PromptRules...)
	}
	targetIdx := -1
	if body.Index != nil && *body.Index >= 0 && *body.Index < len(cur) {
		targetIdx = *body.Index
	} else if body.Match != nil {
		match := strings.TrimSpace(*body.Match)
		if match != "" {
			for i := range cur {
				if cur[i].Name == match {
					targetIdx = i
					break
				}
			}
		}
	}
	if targetIdx >= 0 {
		cur[targetIdx] = *body.Value
	} else {
		cur = append(cur, *body.Value)
	}
	candidate := config.Config{PromptRules: cur}
	candidate.NormalizePromptRules()
	if err := candidate.ValidatePromptRules(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.applyPromptRulesAndPersist(c, candidate.PromptRules)
}

// DeletePromptRule removes a rule by ?name= or ?index=.
func (h *Handler) DeletePromptRule(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	curCfg := h.cfg()
	curRules := make([]config.PromptRule, 0)
	if curCfg != nil {
		curRules = append(curRules, curCfg.PromptRules...)
	}
	if name := strings.TrimSpace(c.Query("name")); name != "" {
		out := make([]config.PromptRule, 0, len(curRules))
		removed := false
		for _, r := range curRules {
			if r.Name == name {
				removed = true
				continue
			}
			out = append(out, r)
		}
		if !removed {
			c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
			return
		}
		h.applyPromptRulesAndPersist(c, out)
		return
	}
	if idxStr := strings.TrimSpace(c.Query("index")); idxStr != "" {
		// strconv.Atoi rejects trailing non-digit junk ("123foo") which
		// fmt.Sscanf silently accepts as 123 — important here because we
		// use the parsed index to splice the slice without further bounds
		// checking on the raw string.
		if idx, err := strconv.Atoi(idxStr); err == nil && idx >= 0 && idx < len(curRules) {
			next := append([]config.PromptRule(nil), curRules[:idx]...)
			next = append(next, curRules[idx+1:]...)
			h.applyPromptRulesAndPersist(c, next)
			return
		}
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "missing name or index"})
}

// applyPromptRulesAndPersist publishes a cloned config snapshot with the new
// prompt-rules list, updates the in-process regex snapshot, and persists the
// cloned config to disk. On persist failure, the runtime snapshot is rolled
// back to the previously-loaded rule list.
//
// Caller MUST already hold h.mu.
func (h *Handler) applyPromptRulesAndPersist(c *gin.Context, next []config.PromptRule) {
	cur := h.cfg()
	prev := make([]config.PromptRule, 0)
	if cur != nil {
		prev = append(prev, cur.PromptRules...)
	}

	clone, err := cloneConfigSnapshot(cur)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clone config"})
		return
	}
	clone.PromptRules = append([]config.PromptRule(nil), next...)
	helps.UpdatePromptRulesSnapshot(clone.PromptRules)

	if err := config.SaveConfigPreserveComments(h.configFilePath, clone); err != nil {
		helps.UpdatePromptRulesSnapshot(prev)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	h.cfgPtr.Store(clone)
	if commit := h.loadCommit(); commit != nil {
		commit(clone)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func readPromptRulesBody(c *gin.Context) ([]config.PromptRule, bool) {
	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return nil, false
	}
	if len(data) == 0 {
		// Empty body explicitly clears the list.
		return []config.PromptRule{}, true
	}
	var arr []config.PromptRule
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, true
	}
	var obj struct {
		Items       []config.PromptRule `json:"items"`
		PromptRules []config.PromptRule `json:"prompt-rules"`
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		if obj.PromptRules != nil {
			return obj.PromptRules, true
		}
		if obj.Items != nil {
			return obj.Items, true
		}
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
	return nil, false
}
