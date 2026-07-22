package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/interfaces"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

type configuredRouteCaptureExecutor struct {
	modelExecutionCaptureExecutor
	mu     sync.Mutex
	models []string
}

func (e *configuredRouteCaptureExecutor) Execute(ctx context.Context, auth *coreauth.Auth, req coreexecutor.Request, opts coreexecutor.Options) (coreexecutor.Response, error) {
	e.mu.Lock()
	e.models = append(e.models, req.Model)
	e.mu.Unlock()
	if e.modelExecutionCaptureExecutor.execute != nil {
		return e.modelExecutionCaptureExecutor.execute(ctx, auth, req, opts)
	}
	return coreexecutor.Response{Payload: []byte(fmt.Sprintf(`{"model":%q}`, req.Model))}, nil
}

func (e *configuredRouteCaptureExecutor) ExecuteStream(ctx context.Context, auth *coreauth.Auth, req coreexecutor.Request, opts coreexecutor.Options) (*coreexecutor.StreamResult, error) {
	e.mu.Lock()
	e.models = append(e.models, req.Model)
	e.mu.Unlock()
	return e.modelExecutionCaptureExecutor.ExecuteStream(ctx, auth, req, opts)
}

func (e *configuredRouteCaptureExecutor) executedModels() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.models...)
}

func newConfiguredRouteHandler(t *testing.T, cfg *sdkconfig.SDKConfig, executor *configuredRouteCaptureExecutor, models ...string) *BaseAPIHandler {
	t.Helper()
	manager := coreauth.NewManager(nil, nil, nil)
	manager.RegisterExecutor(executor)
	for _, model := range models {
		auth := &coreauth.Auth{
			ID:       "configured-route-" + model,
			Provider: executor.Identifier(),
			Status:   coreauth.StatusActive,
			Metadata: map[string]any{"email": model + "@example.com"},
		}
		if _, errRegister := manager.Register(context.Background(), auth); errRegister != nil {
			t.Fatalf("manager.Register(%s): %v", model, errRegister)
		}
		registry.GetGlobalRegistry().RegisterClient(auth.ID, auth.Provider, []*registry.ModelInfo{{ID: model}})
		t.Cleanup(func() {
			registry.GetGlobalRegistry().UnregisterClient(auth.ID)
		})
	}
	return NewBaseAPIHandlers(cfg, manager)
}

func TestConfiguredModelRoutePriorityFailsOverAndPreservesRequestedSuffix(t *testing.T) {
	modelA := "configured-route-priority-a"
	modelB := "configured-route-priority-b"
	executor := &configuredRouteCaptureExecutor{}
	executor.execute = func(ctx context.Context, auth *coreauth.Auth, req coreexecutor.Request, opts coreexecutor.Options) (coreexecutor.Response, error) {
		if req.Model == modelA+"(high)" {
			return coreexecutor.Response{}, &coreauth.Error{HTTPStatus: http.StatusTooManyRequests, Message: "quota"}
		}
		return coreexecutor.Response{Payload: []byte(fmt.Sprintf(`{"model":%q,"ok":true}`, req.Model))}, nil
	}
	handler := newConfiguredRouteHandler(t, &sdkconfig.SDKConfig{ModelRoutes: []sdkconfig.ModelRoute{{
		Alias:    "auto",
		Strategy: sdkconfig.ModelRouteStrategyPriority,
		Models:   []string{modelA, modelB},
	}}}, executor, modelA, modelB)

	body, _, errMsg := handler.ExecuteWithAuthManager(context.Background(), "openai", "auto(high)", []byte(`{"model":"auto(high)"}`), "")
	if errMsg != nil {
		t.Fatalf("ExecuteWithAuthManager() error = %+v", errMsg)
	}
	if got := string(body); got != `{"model":"auto(high)","ok":true}` {
		t.Fatalf("body = %s", got)
	}
	gotModels := executor.executedModels()
	wantModels := []string{modelA + "(high)", modelB + "(high)"}
	if fmt.Sprint(gotModels) != fmt.Sprint(wantModels) {
		t.Fatalf("executed models = %v, want %v", gotModels, wantModels)
	}
}

func TestConfiguredModelRouteLateStreamFailureCoolsTargetForNextRequest(t *testing.T) {
	modelA := "configured-route-stream-a"
	modelB := "configured-route-stream-b"
	route := sdkconfig.ModelRoute{
		Alias:           "auto",
		Strategy:        sdkconfig.ModelRouteStrategyPriority,
		CooldownSeconds: 60,
		Models:          []string{modelA, modelB},
	}
	executor := &configuredRouteCaptureExecutor{}
	executor.stream = func(_ context.Context, _ *coreauth.Auth, req coreexecutor.Request, _ coreexecutor.Options) (*coreexecutor.StreamResult, error) {
		chunks := make(chan coreexecutor.StreamChunk, 2)
		chunks <- coreexecutor.StreamChunk{Payload: []byte(fmt.Sprintf(`{"model":%q,"partial":true}`, req.Model))}
		if req.Model == modelA {
			chunks <- coreexecutor.StreamChunk{Err: &coreauth.Error{
				Code:       "rate_limit_exceeded",
				Message:    "quota",
				Retryable:  true,
				HTTPStatus: http.StatusTooManyRequests,
			}}
		}
		close(chunks)
		return &coreexecutor.StreamResult{Chunks: chunks}, nil
	}
	handler := newConfiguredRouteHandler(t, &sdkconfig.SDKConfig{ModelRoutes: []sdkconfig.ModelRoute{route}}, executor, modelA, modelB)

	dataChan, _, errChan := handler.ExecuteStreamWithAuthManager(context.Background(), "openai", "auto", []byte(`{"model":"auto","stream":true}`), "")
	var firstBody []byte
	for chunk := range dataChan {
		firstBody = append(firstBody, chunk...)
	}
	if len(firstBody) == 0 {
		t.Fatal("first stream returned no payload before the terminal error")
	}
	var firstErr *interfaces.ErrorMessage
	for errMsg := range errChan {
		firstErr = errMsg
	}
	if firstErr == nil || firstErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("first stream error = %+v, want status %d", firstErr, http.StatusTooManyRequests)
	}
	if selection := handler.configuredModelRouteRuntime().Select(route, time.Now()); selection.model != modelB {
		t.Fatalf("selected model after late stream failure = %q, want %q", selection.model, modelB)
	}

	dataChan, _, errChan = handler.ExecuteStreamWithAuthManager(context.Background(), "openai", "auto", []byte(`{"model":"auto","stream":true}`), "")
	for range dataChan {
	}
	for errMsg := range errChan {
		if errMsg != nil {
			t.Fatalf("second stream error = %+v", errMsg)
		}
	}
	gotModels := executor.executedModels()
	wantModels := []string{modelA, modelB}
	if fmt.Sprint(gotModels) != fmt.Sprint(wantModels) {
		t.Fatalf("executed models = %v, want %v", gotModels, wantModels)
	}
}

func TestConfiguredModelRouteRoundRobinRotatesRequests(t *testing.T) {
	modelA := "configured-route-round-a"
	modelB := "configured-route-round-b"
	executor := &configuredRouteCaptureExecutor{}
	handler := newConfiguredRouteHandler(t, &sdkconfig.SDKConfig{ModelRoutes: []sdkconfig.ModelRoute{{
		Alias:    "auto",
		Strategy: sdkconfig.ModelRouteStrategyRoundRobin,
		Models:   []string{modelA, modelB},
	}}}, executor, modelA, modelB)

	for i := 0; i < 2; i++ {
		if _, _, errMsg := handler.ExecuteWithAuthManager(context.Background(), "openai", "auto", []byte(`{"model":"auto"}`), ""); errMsg != nil {
			t.Fatalf("ExecuteWithAuthManager(%d) error = %+v", i, errMsg)
		}
	}
	gotModels := executor.executedModels()
	wantModels := []string{modelA, modelB}
	if fmt.Sprint(gotModels) != fmt.Sprint(wantModels) {
		t.Fatalf("executed models = %v, want %v", gotModels, wantModels)
	}
}

func TestConfiguredModelRouteCandidateSkipsPluginModelRouters(t *testing.T) {
	targetModel := "configured-route-skip-router-target"
	executor := &configuredRouteCaptureExecutor{}
	host := &handlerModelRouterTestHost{hasRouters: true}
	host.route = func(ctx context.Context, req pluginapi.ModelRouteRequest, skipPluginID string) (pluginapi.ModelRouteResponse, bool) {
		if req.RequestedModel == "auto" {
			return pluginapi.ModelRouteResponse{}, false
		}
		t.Fatalf("plugin model router was called for configured route candidate %q", req.RequestedModel)
		return pluginapi.ModelRouteResponse{}, false
	}
	handler := newConfiguredRouteHandler(t, &sdkconfig.SDKConfig{ModelRoutes: []sdkconfig.ModelRoute{{
		Alias:  "auto",
		Models: []string{targetModel},
	}}}, executor, targetModel)
	handler.SetModelRouterHost(host)

	if _, _, errMsg := handler.ExecuteWithAuthManager(context.Background(), "openai", "auto", []byte(`{"model":"auto"}`), ""); errMsg != nil {
		t.Fatalf("ExecuteWithAuthManager() error = %+v", errMsg)
	}
	if gotModels := executor.executedModels(); len(gotModels) != 1 || gotModels[0] != targetModel {
		t.Fatalf("executed models = %v, want [%s]", gotModels, targetModel)
	}
}
