package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/thinking"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

type configuredModelRouteRuntime struct {
	mu     sync.Mutex
	states map[string]*configuredModelRouteState
}

type configuredModelRouteState struct {
	signature string
	cursor    int
	cooldowns map[string]time.Time
}

type configuredModelRouteSelection struct {
	model      string
	allCooling bool
	retryAfter time.Duration
}

type requestScopedError interface {
	IsRequestScoped() bool
}

func newConfiguredModelRouteRuntime(cfg *config.SDKConfig) *configuredModelRouteRuntime {
	rt := &configuredModelRouteRuntime{states: make(map[string]*configuredModelRouteState)}
	rt.Sync(cfg)
	return rt
}

func (rt *configuredModelRouteRuntime) Sync(cfg *config.SDKConfig) {
	if rt == nil {
		return
	}
	routes := configuredModelRoutes(cfg)
	next := make(map[string]*configuredModelRouteState, len(routes))
	rt.mu.Lock()
	defer rt.mu.Unlock()
	for _, route := range routes {
		key := modelRouteKey(route.Alias)
		if key == "" {
			continue
		}
		signature := modelRouteSignature(route)
		state := rt.states[key]
		if state == nil || state.signature != signature {
			state = &configuredModelRouteState{
				signature: signature,
				cooldowns: make(map[string]time.Time),
			}
		}
		next[key] = state
	}
	rt.states = next
}

func (rt *configuredModelRouteRuntime) Select(route config.ModelRoute, now time.Time) configuredModelRouteSelection {
	if rt == nil {
		return configuredModelRouteSelection{}
	}
	key := modelRouteKey(route.Alias)
	if key == "" || len(route.Models) == 0 {
		return configuredModelRouteSelection{}
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	state := rt.stateLocked(route)
	if state == nil {
		return configuredModelRouteSelection{}
	}
	models := route.Models
	switch strings.ToLower(strings.TrimSpace(route.Strategy)) {
	case config.ModelRouteStrategyRoundRobin:
		start := state.cursor
		if start < 0 || start >= len(models) {
			start = 0
		}
		var earliest time.Time
		for offset := 0; offset < len(models); offset++ {
			idx := (start + offset) % len(models)
			model := strings.TrimSpace(models[idx])
			if model == "" {
				continue
			}
			until := state.cooldowns[modelRouteKey(model)]
			if until.IsZero() || !now.Before(until) {
				state.cursor = (idx + 1) % len(models)
				return configuredModelRouteSelection{model: model}
			}
			earliest = earliestCooldown(earliest, until)
		}
		return configuredModelRouteSelection{allCooling: !earliest.IsZero(), retryAfter: cooldownRetryAfter(now, earliest)}
	default:
		var earliest time.Time
		for _, model := range models {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			until := state.cooldowns[modelRouteKey(model)]
			if until.IsZero() || !now.Before(until) {
				return configuredModelRouteSelection{model: model}
			}
			earliest = earliestCooldown(earliest, until)
		}
		return configuredModelRouteSelection{allCooling: !earliest.IsZero(), retryAfter: cooldownRetryAfter(now, earliest)}
	}
}

func (rt *configuredModelRouteRuntime) MarkFailure(route config.ModelRoute, model string, errMsg *interfaces.ErrorMessage, now time.Time) {
	if rt == nil {
		return
	}
	key := modelRouteKey(route.Alias)
	modelKey := modelRouteKey(model)
	if key == "" || modelKey == "" {
		return
	}
	cooldown := time.Duration(route.CooldownSeconds) * time.Second
	if cooldown <= 0 {
		cooldown = time.Duration(config.DefaultModelRouteCooldownSeconds) * time.Second
	}
	if retryAfter := retryAfterDuration(errMsg, now); retryAfter > cooldown {
		cooldown = retryAfter
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	state := rt.stateLocked(route)
	if state == nil {
		return
	}
	state.cooldowns[modelKey] = now.Add(cooldown)
}

func (rt *configuredModelRouteRuntime) stateLocked(route config.ModelRoute) *configuredModelRouteState {
	if rt == nil {
		return nil
	}
	if rt.states == nil {
		rt.states = make(map[string]*configuredModelRouteState)
	}
	key := modelRouteKey(route.Alias)
	if key == "" {
		return nil
	}
	signature := modelRouteSignature(route)
	state := rt.states[key]
	if state == nil || state.signature != signature {
		state = &configuredModelRouteState{
			signature: signature,
			cooldowns: make(map[string]time.Time),
		}
		rt.states[key] = state
	}
	return state
}

func (h *BaseAPIHandler) configuredModelRouteForExecution(modelName string) (config.ModelRoute, bool) {
	if h == nil || h.Cfg == nil {
		return config.ModelRoute{}, false
	}
	requested := strings.TrimSpace(modelName)
	if requested == "" {
		return config.ModelRoute{}, false
	}
	parsed := thinking.ParseSuffix(requested)
	alias := strings.TrimSpace(parsed.ModelName)
	for _, route := range configuredModelRoutes(h.Cfg) {
		if strings.EqualFold(strings.TrimSpace(route.Alias), alias) {
			return route, true
		}
	}
	return config.ModelRoute{}, false
}

func configuredModelRoutes(cfg *config.SDKConfig) []config.ModelRoute {
	if cfg == nil || len(cfg.ModelRoutes) == 0 {
		return nil
	}
	out := make([]config.ModelRoute, 0, len(cfg.ModelRoutes))
	for _, route := range cfg.ModelRoutes {
		route.Alias = strings.TrimSpace(route.Alias)
		route.Strategy = strings.ToLower(strings.TrimSpace(route.Strategy))
		if route.Strategy == "" {
			route.Strategy = config.ModelRouteStrategyPriority
		}
		if route.CooldownSeconds <= 0 {
			route.CooldownSeconds = config.DefaultModelRouteCooldownSeconds
		}
		models := make([]string, 0, len(route.Models))
		for _, model := range route.Models {
			model = strings.TrimSpace(model)
			if model != "" {
				models = append(models, model)
			}
		}
		route.Models = models
		if route.Alias != "" && len(route.Models) > 0 {
			out = append(out, route)
		}
	}
	return out
}

func (h *BaseAPIHandler) executeConfiguredModelRoute(ctx context.Context, entryProtocol, exitProtocol string, route config.ModelRoute, requestedModel string, rawJSON []byte, alt string, parentOptions modelExecutionOptions) ([]byte, http.Header, *interfaces.ErrorMessage) {
	runtime := h.configuredModelRouteRuntime()
	var lastErr *interfaces.ErrorMessage
	for attempt := 0; attempt < len(route.Models); attempt++ {
		selection := runtime.Select(route, time.Now())
		if selection.allCooling {
			return nil, nil, configuredModelRouteCooldownError(route, selection.retryAfter)
		}
		if selection.model == "" {
			break
		}
		targetModel := modelRouteTargetModel(requestedModel, selection.model)
		body, headers, errMsg := h.executeWithAuthManagerFormats(ctx, entryProtocol, exitProtocol, targetModel, rawJSON, alt, false, configuredModelRouteChildOptions(parentOptions, requestedModel))
		if errMsg == nil {
			return rewriteConfiguredModelRouteBody(body, requestedModel), headers, nil
		}
		if !shouldConfiguredModelRouteFailover(ctx, errMsg) {
			return nil, nil, errMsg
		}
		lastErr = errMsg
		runtime.MarkFailure(route, selection.model, errMsg, time.Now())
	}
	return nil, nil, configuredModelRouteUnavailableError(route, lastErr)
}

func (h *BaseAPIHandler) countConfiguredModelRoute(ctx context.Context, handlerType string, route config.ModelRoute, requestedModel string, rawJSON []byte, alt string, parentOptions modelExecutionOptions) ([]byte, http.Header, *interfaces.ErrorMessage) {
	runtime := h.configuredModelRouteRuntime()
	var lastErr *interfaces.ErrorMessage
	for attempt := 0; attempt < len(route.Models); attempt++ {
		selection := runtime.Select(route, time.Now())
		if selection.allCooling {
			return nil, nil, configuredModelRouteCooldownError(route, selection.retryAfter)
		}
		if selection.model == "" {
			break
		}
		targetModel := modelRouteTargetModel(requestedModel, selection.model)
		body, headers, errMsg := h.executeCountWithAuthManager(ctx, handlerType, targetModel, rawJSON, alt, configuredModelRouteChildOptions(parentOptions, requestedModel))
		if errMsg == nil {
			return rewriteConfiguredModelRouteBody(body, requestedModel), headers, nil
		}
		if !shouldConfiguredModelRouteFailover(ctx, errMsg) {
			return nil, nil, errMsg
		}
		lastErr = errMsg
		runtime.MarkFailure(route, selection.model, errMsg, time.Now())
	}
	return nil, nil, configuredModelRouteUnavailableError(route, lastErr)
}

func (h *BaseAPIHandler) streamConfiguredModelRoute(ctx context.Context, entryProtocol, exitProtocol string, route config.ModelRoute, requestedModel string, rawJSON []byte, alt string, parentOptions modelExecutionOptions) (<-chan []byte, http.Header, <-chan *interfaces.ErrorMessage) {
	runtime := h.configuredModelRouteRuntime()
	var lastErr *interfaces.ErrorMessage
	for attempt := 0; attempt < len(route.Models); attempt++ {
		selection := runtime.Select(route, time.Now())
		if selection.allCooling {
			return errorStreamResult(configuredModelRouteCooldownError(route, selection.retryAfter))
		}
		if selection.model == "" {
			break
		}
		targetModel := modelRouteTargetModel(requestedModel, selection.model)
		dataChan, headers, errChan := h.executeStreamWithAuthManagerFormats(ctx, entryProtocol, exitProtocol, targetModel, rawJSON, alt, false, configuredModelRouteChildOptions(parentOptions, requestedModel))
		firstChunk, gotFirstChunk, errMsg := receiveConfiguredModelRouteFirstChunk(ctx, dataChan, errChan)
		if errMsg == nil {
			outData, outErr := rewriteConfiguredModelRouteStream(ctx, firstChunk, gotFirstChunk, dataChan, errChan, requestedModel, func(lateErr *interfaces.ErrorMessage) {
				if shouldConfiguredModelRouteFailover(ctx, lateErr) {
					runtime.MarkFailure(route, selection.model, lateErr, time.Now())
				}
			})
			return outData, headers, outErr
		}
		if !shouldConfiguredModelRouteFailover(ctx, errMsg) {
			return errorStreamResult(errMsg)
		}
		lastErr = errMsg
		runtime.MarkFailure(route, selection.model, errMsg, time.Now())
	}
	return errorStreamResult(configuredModelRouteUnavailableError(route, lastErr))
}

func (h *BaseAPIHandler) configuredModelRouteRuntime() *configuredModelRouteRuntime {
	if h == nil {
		return newConfiguredModelRouteRuntime(nil)
	}
	if h.modelRouteRuntime == nil {
		h.modelRouteRuntime = newConfiguredModelRouteRuntime(h.Cfg)
	}
	return h.modelRouteRuntime
}

func configuredModelRouteChildOptions(parent modelExecutionOptions, requestedModel string) modelExecutionOptions {
	parent.SkipConfiguredModelRoute = true
	parent.DisablePluginModelRouter = true
	parent.RequestedModelOverride = requestedModel
	return parent
}

func modelRouteTargetModel(requestedModel, targetModel string) string {
	targetModel = strings.TrimSpace(targetModel)
	targetSuffix := thinking.ParseSuffix(targetModel)
	if targetSuffix.HasSuffix {
		return targetModel
	}
	requestSuffix := thinking.ParseSuffix(strings.TrimSpace(requestedModel))
	if requestSuffix.HasSuffix {
		return fmt.Sprintf("%s(%s)", targetModel, requestSuffix.RawSuffix)
	}
	return targetModel
}

func rewriteConfiguredModelRouteBody(body []byte, requestedModel string) []byte {
	return coreauth.RewriteModelInResponse(body, strings.TrimSpace(requestedModel))
}

func receiveConfiguredModelRouteFirstChunk(ctx context.Context, dataChan <-chan []byte, errChan <-chan *interfaces.ErrorMessage) ([]byte, bool, *interfaces.ErrorMessage) {
	for dataChan != nil || errChan != nil {
		select {
		case <-ctx.Done():
			return nil, false, &interfaces.ErrorMessage{StatusCode: http.StatusRequestTimeout, Error: ctx.Err()}
		case chunk, ok := <-dataChan:
			if !ok {
				dataChan = nil
				continue
			}
			return chunk, true, nil
		case errMsg, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			if errMsg != nil {
				return nil, false, errMsg
			}
		}
	}
	return nil, false, &interfaces.ErrorMessage{StatusCode: http.StatusBadGateway, Error: errors.New("upstream stream closed before emitting data")}
}

func rewriteConfiguredModelRouteStream(ctx context.Context, firstChunk []byte, gotFirstChunk bool, dataChan <-chan []byte, errChan <-chan *interfaces.ErrorMessage, requestedModel string, onTerminalError func(*interfaces.ErrorMessage)) (<-chan []byte, <-chan *interfaces.ErrorMessage) {
	outData := make(chan []byte)
	outErr := make(chan *interfaces.ErrorMessage, 1)
	go func() {
		defer close(outData)
		defer close(outErr)
		rewriter := coreauth.NewStreamRewriter(coreauth.StreamRewriteOptions{RewriteModel: strings.TrimSpace(requestedModel)})
		send := func(chunk []byte) bool {
			rewritten := rewriter.RewriteChunk(chunk)
			if len(rewritten) == 0 {
				return true
			}
			select {
			case outData <- rewritten:
				return true
			case <-ctx.Done():
				return false
			}
		}
		if gotFirstChunk && !send(firstChunk) {
			return
		}
		for dataChan != nil || errChan != nil {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-dataChan:
				if !ok {
					dataChan = nil
					continue
				}
				if !send(chunk) {
					return
				}
			case errMsg, ok := <-errChan:
				if !ok {
					errChan = nil
					continue
				}
				if errMsg != nil {
					if onTerminalError != nil {
						onTerminalError(errMsg)
					}
					select {
					case outErr <- errMsg:
					case <-ctx.Done():
					}
					return
				}
			}
		}
	}()
	return outData, outErr
}

func errorStreamResult(errMsg *interfaces.ErrorMessage) (<-chan []byte, http.Header, <-chan *interfaces.ErrorMessage) {
	errChan := make(chan *interfaces.ErrorMessage, 1)
	errChan <- errMsg
	close(errChan)
	return nil, nil, errChan
}

func shouldConfiguredModelRouteFailover(ctx context.Context, errMsg *interfaces.ErrorMessage) bool {
	if errMsg == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if errMsg.Error != nil {
		if errors.Is(errMsg.Error, context.Canceled) || errors.Is(errMsg.Error, context.DeadlineExceeded) {
			return false
		}
		var scoped requestScopedError
		if errors.As(errMsg.Error, &scoped) && scoped != nil && scoped.IsRequestScoped() {
			return false
		}
	}
	status := errMsg.StatusCode
	if status <= 0 {
		return true
	}
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return false
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden, http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	case http.StatusNotFound:
		return !isRequestScopedNotFound(errMsg)
	default:
		return status >= http.StatusInternalServerError
	}
}

func isRequestScopedNotFound(errMsg *interfaces.ErrorMessage) bool {
	if errMsg == nil || errMsg.Error == nil {
		return false
	}
	message := strings.ToLower(errMsg.Error.Error())
	return strings.Contains(message, "items are not persisted") ||
		strings.Contains(message, "store") && strings.Contains(message, "false")
}

func configuredModelRouteCooldownError(route config.ModelRoute, retryAfter time.Duration) *interfaces.ErrorMessage {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Retry-After", strconv.Itoa(seconds))
	return &interfaces.ErrorMessage{
		StatusCode: http.StatusTooManyRequests,
		Error:      errors.New(modelRouteErrorJSON("model_route_cooldown", fmt.Sprintf("all candidates for model route %q are cooling down", strings.TrimSpace(route.Alias)), route.Alias, "")),
		Addon:      headers,
	}
}

func configuredModelRouteUnavailableError(route config.ModelRoute, lastErr *interfaces.ErrorMessage) *interfaces.ErrorMessage {
	detail := ""
	if lastErr != nil && lastErr.Error != nil {
		detail = lastErr.Error.Error()
	}
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	return &interfaces.ErrorMessage{
		StatusCode: http.StatusServiceUnavailable,
		Error:      errors.New(modelRouteErrorJSON("model_route_unavailable", fmt.Sprintf("no available candidate for model route %q", strings.TrimSpace(route.Alias)), route.Alias, detail)),
		Addon:      headers,
	}
}

func modelRouteErrorJSON(code, message, alias, detail string) string {
	payload := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"alias":   strings.TrimSpace(alias),
		},
	}
	if strings.TrimSpace(detail) != "" {
		payload["error"].(map[string]any)["detail"] = detail
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return message
	}
	return string(data)
}

func retryAfterDuration(errMsg *interfaces.ErrorMessage, now time.Time) time.Duration {
	if errMsg == nil || errMsg.Addon == nil {
		return 0
	}
	value := strings.TrimSpace(errMsg.Addon.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if parsed, err := http.ParseTime(value); err == nil {
		if parsed.After(now) {
			return parsed.Sub(now)
		}
	}
	return 0
}

func modelRouteSignature(route config.ModelRoute) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(strings.TrimSpace(route.Alias)))
	b.WriteByte('\n')
	b.WriteString(strings.ToLower(strings.TrimSpace(route.Strategy)))
	b.WriteByte('\n')
	b.WriteString(strconv.Itoa(route.CooldownSeconds))
	for _, model := range route.Models {
		b.WriteByte('\n')
		b.WriteString(strings.TrimSpace(model))
	}
	return b.String()
}

func modelRouteKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func earliestCooldown(current, candidate time.Time) time.Time {
	if candidate.IsZero() {
		return current
	}
	if current.IsZero() || candidate.Before(current) {
		return candidate
	}
	return current
}

func cooldownRetryAfter(now, until time.Time) time.Duration {
	if until.IsZero() || !until.After(now) {
		return 0
	}
	return until.Sub(now)
}
