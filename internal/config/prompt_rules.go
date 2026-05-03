package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// PromptRule constants — exported for consumers to avoid string typos.
const (
	PromptRuleTargetSystem = "system"
	PromptRuleTargetUser   = "user"

	PromptRuleActionInject = "inject"
	PromptRuleActionStrip  = "strip"

	PromptRulePositionAppend  = "append"
	PromptRulePositionPrepend = "prepend"

	// PromptRulePatternMaxLen caps strip-rule regex pattern length to mitigate
	// memory blowup risk even though Go's RE2 is linear-time.
	PromptRulePatternMaxLen = 1024
)

// PromptRule describes a content-level transformation applied pre-translation to
// either the system prompt or the most recent natural-language user message of an
// outgoing request, scoped by model glob and source format.
//
// Inject rules write Content into the target idempotently. Marker is optional
// and toggles between two semantics:
//   - When Marker is empty (boundary mode), Position is relative to the target
//     text boundaries (append = end, prepend = start) and idempotency is
//     "skip when target already contains Content as a substring".
//   - When Marker is non-empty (anchor mode), Position is relative to the
//     marker (append = immediately after, prepend = immediately before),
//     idempotency is "skip when Content is already directly adjacent to some
//     marker occurrence in the configured direction", and the rule no-ops if
//     the marker is not present in the target.
//
// Strip rules apply an RE2 regex to the target's text, replacing matches with
// empty string. Strip rules run before inject rules within a single request so
// injected content is never accidentally stripped on the same pass.
type PromptRule struct {
	// Name uniquely identifies the rule for management API operations.
	Name string `yaml:"name" json:"name"`
	// Enabled toggles whether the rule fires.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Models scopes the rule. Empty list means match all models and source formats.
	// Reuses PayloadModelRule; the Protocol field here means SOURCE format (e.g.,
	// "openai", "openai-response", "claude", "gemini", "gemini-cli").
	Models []PayloadModelRule `yaml:"models,omitempty" json:"models,omitempty"`
	// Target is "system" or "user".
	Target string `yaml:"target" json:"target"`
	// Action is "inject" or "strip".
	Action string `yaml:"action" json:"action"`
	// Content is the literal text injected. Required for inject.
	Content string `yaml:"content,omitempty" json:"content,omitempty"`
	// Marker is optional. When non-empty it acts as an anchor: Position is
	// relative to the marker and idempotency is checked by adjacency. When
	// empty, Position is relative to the target's text boundaries and
	// idempotency is checked by Content presence.
	Marker string `yaml:"marker,omitempty" json:"marker,omitempty"`
	// Position is "prepend" or "append" (default "append"). Used by inject.
	Position string `yaml:"position,omitempty" json:"position,omitempty"`
	// Pattern is the RE2 regex used by strip. Length capped at PromptRulePatternMaxLen.
	Pattern string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
}

// promptRulesUpdateHook is invoked by SanitizePromptRules so the runtime cache of
// compiled regexes stays in sync with the loaded config. Set via
// SetPromptRulesUpdateHook from the runtime package; nil by default to keep config
// usable in isolation (and to avoid an import cycle).
var promptRulesUpdateHook func([]PromptRule)

// promptRuleProtocolValidator is set by the runtime package to vet that
// PromptRule.Models[].Protocol values are in the allowed source-format set.
// Returning false rejects the rule. nil means "any protocol accepted".
var promptRuleProtocolValidator func(string) bool

// SetPromptRuleProtocolValidator registers a predicate used by ValidatePromptRule
// to reject rules whose Models[].Protocol is not a recognized source format.
// Wired from internal/runtime/executor/helps to keep config decoupled.
func SetPromptRuleProtocolValidator(fn func(string) bool) {
	promptRuleProtocolValidator = fn
}

// SetPromptRulesUpdateHook registers a callback invoked after SanitizePromptRules
// completes. The runtime package uses this to rebuild its compiled-regex snapshot
// without introducing a config→helps import.
func SetPromptRulesUpdateHook(fn func([]PromptRule)) {
	promptRulesUpdateHook = fn
}

func notifyPromptRulesUpdated(rules []PromptRule) {
	if promptRulesUpdateHook == nil {
		return
	}
	cp := append([]PromptRule(nil), rules...)
	promptRulesUpdateHook(cp)
}

// SanitizePromptRules normalizes every prompt rule and drops any that fail
// validation (or duplicate an earlier name), with a debug log explaining why.
// Used at config load so a malformed YAML on disk does not block startup.
//
// Always invokes the registered update hook — even when the input list is empty
// or nil — so the runtime cache is cleared when an admin removes all rules via
// YAML upload (per Codex post-impl review BLOCKER #7).
func (cfg *Config) SanitizePromptRules() {
	if cfg == nil {
		return
	}
	if len(cfg.PromptRules) == 0 {
		notifyPromptRulesUpdated(nil)
		return
	}
	out := make([]PromptRule, 0, len(cfg.PromptRules))
	seen := make(map[string]struct{}, len(cfg.PromptRules))
	for i := range cfg.PromptRules {
		rule := normalizePromptRule(cfg.PromptRules[i])
		if err := validatePromptRule(&rule); err != nil {
			log.WithFields(log.Fields{
				"section":    "prompt-rules",
				"rule_index": i + 1,
				"rule_name":  rule.Name,
				"error":      err.Error(),
			}).Warn("prompt rule dropped: invalid")
			continue
		}
		if _, dup := seen[rule.Name]; dup {
			log.WithFields(log.Fields{
				"section":    "prompt-rules",
				"rule_index": i + 1,
				"rule_name":  rule.Name,
			}).Warn("prompt rule dropped: duplicate name")
			continue
		}
		seen[rule.Name] = struct{}{}
		out = append(out, rule)
	}
	cfg.PromptRules = out
	notifyPromptRulesUpdated(cfg.PromptRules)
}

// ValidatePromptRules returns the first validation error encountered, or nil when
// every rule is well-formed. Used at API write paths so the caller receives a 400
// with a reason instead of silently dropping rules. Names must be unique across
// the list since they identify rules for PATCH/DELETE operations.
func (cfg *Config) ValidatePromptRules() error {
	if cfg == nil {
		return nil
	}
	seen := make(map[string]int, len(cfg.PromptRules))
	for i := range cfg.PromptRules {
		rule := normalizePromptRule(cfg.PromptRules[i])
		if err := validatePromptRule(&rule); err != nil {
			return fmt.Errorf("prompt-rules[%d] %q: %w", i, rule.Name, err)
		}
		if prev, dup := seen[rule.Name]; dup {
			return fmt.Errorf("prompt-rules[%d] %q: duplicate name (also at index %d)", i, rule.Name, prev)
		}
		seen[rule.Name] = i
	}
	return nil
}

// NormalizePromptRules applies trim/lowercase/default-position normalization in
// place so the persisted config matches what passes validation. Call after a
// successful ValidatePromptRules at write paths before persisting.
func (cfg *Config) NormalizePromptRules() {
	if cfg == nil {
		return
	}
	for i := range cfg.PromptRules {
		cfg.PromptRules[i] = normalizePromptRule(cfg.PromptRules[i])
	}
}

func normalizePromptRule(r PromptRule) PromptRule {
	r.Name = strings.TrimSpace(r.Name)
	r.Target = strings.ToLower(strings.TrimSpace(r.Target))
	r.Action = strings.ToLower(strings.TrimSpace(r.Action))
	r.Marker = strings.TrimSpace(r.Marker)
	if r.Action == PromptRuleActionInject {
		pos := strings.ToLower(strings.TrimSpace(r.Position))
		if pos == "" {
			pos = PromptRulePositionAppend
		}
		r.Position = pos
	} else {
		r.Position = ""
	}
	for j := range r.Models {
		r.Models[j].Name = strings.TrimSpace(r.Models[j].Name)
		r.Models[j].Protocol = strings.ToLower(strings.TrimSpace(r.Models[j].Protocol))
	}
	return r
}

func validatePromptRule(r *PromptRule) error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	for j := range r.Models {
		if promptRuleProtocolValidator != nil && !promptRuleProtocolValidator(r.Models[j].Protocol) {
			return fmt.Errorf("models[%d].protocol %q is not a recognized source format", j, r.Models[j].Protocol)
		}
	}
	switch r.Target {
	case PromptRuleTargetSystem, PromptRuleTargetUser:
	default:
		return fmt.Errorf("invalid target %q (expected %q or %q)", r.Target, PromptRuleTargetSystem, PromptRuleTargetUser)
	}
	switch r.Action {
	case PromptRuleActionInject:
		if r.Content == "" {
			return errors.New("content is required for inject")
		}
		switch r.Position {
		case PromptRulePositionAppend, PromptRulePositionPrepend:
		default:
			return fmt.Errorf("invalid position %q (expected %q or %q)", r.Position, PromptRulePositionAppend, PromptRulePositionPrepend)
		}
		if r.Pattern != "" {
			return errors.New("pattern must be empty for inject")
		}
	case PromptRuleActionStrip:
		if r.Pattern == "" {
			return errors.New("pattern is required for strip")
		}
		if len(r.Pattern) > PromptRulePatternMaxLen {
			return fmt.Errorf("pattern length %d exceeds max %d", len(r.Pattern), PromptRulePatternMaxLen)
		}
		if _, err := regexp.Compile(r.Pattern); err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
		if r.Content != "" || r.Marker != "" {
			return errors.New("content and marker must be empty for strip")
		}
	default:
		return fmt.Errorf("invalid action %q (expected %q or %q)", r.Action, PromptRuleActionInject, PromptRuleActionStrip)
	}
	return nil
}
