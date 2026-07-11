package cliproxy

import (
	"sync"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

// TestService_ConfigSnapshot_RaceFree covers complete executor and model
// registration operations while hot reload publishes new immutable configs.
func TestService_ConfigSnapshot_RaceFree(t *testing.T) {
	s := &Service{
		coreManager: coreauth.NewManager(nil, nil, nil),
		cfg: &config.Config{
			ClaudeKey: []config.ClaudeKey{
				{APIKey: "key-A", BaseURL: "https://a.example.com"},
			},
			GeminiKey: []config.GeminiKey{
				{APIKey: "gem-A", BaseURL: "https://gem-a.example.com"},
			},
			VertexCompatAPIKey: []config.VertexCompatKey{
				{APIKey: "vc-A", BaseURL: "https://vc-a.example.com"},
			},
			CodexKey: []config.CodexKey{
				{APIKey: "cx-A", BaseURL: "https://cx-a.example.com"},
			},
			OAuthExcludedModels: map[string][]string{
				"claude": {"claude-old-model"},
			},
			OpenAICompatibility: []config.OpenAICompatibility{{
				Name: "compat", Models: []config.OpenAICompatibilityModel{{Name: "model-a"}},
			}},
		},
	}

	auth := &coreauth.Auth{
		ID:       "auth-id",
		Provider: "claude",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"auth_kind": "apikey",
			"api_key":   "key-A",
			"base_url":  "https://a.example.com",
		},
	}
	compatAuth := &coreauth.Auth{
		ID:       "compat-auth",
		Provider: "openai-compatibility",
		Attributes: map[string]string{
			"auth_kind":    "apikey",
			"compat_name":  "compat",
			"provider_key": "compat",
		},
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	readers := []func(){
		func() { _ = s.resolveConfigClaudeKey(auth) },
		func() { _ = s.resolveConfigGeminiKey(auth) },
		func() { _ = s.resolveConfigVertexCompatKey(auth) },
		func() { _ = s.resolveConfigCodexKey(auth) },
		func() { _ = s.oauthExcludedModels("claude", "oauth") },
		func() { s.registerExecutorsForAuths([]*coreauth.Auth{auth, compatAuth}, true) },
		func() { s.registerModelsForAuth(nil, auth) },
		func() { s.registerModelsForAuth(nil, compatAuth) },
		func() { _ = s.hasNativeOpenAICompatExecutorConfig(compatAuth, "compat") },
	}
	for i := 0; i < 4; i++ {
		for _, fn := range readers {
			wg.Add(1)
			f := fn
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						f()
					}
				}
			}()
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		flip := false
		for {
			select {
			case <-stop:
				return
			default:
				flip = !flip
				suffix := "A"
				if flip {
					suffix = "B"
				}
				newCfg := &config.Config{
					ClaudeKey: []config.ClaudeKey{
						{APIKey: "key-" + suffix, BaseURL: "https://" + suffix + ".example.com"},
					},
					GeminiKey: []config.GeminiKey{
						{APIKey: "gem-" + suffix, BaseURL: "https://gem-" + suffix + ".example.com"},
					},
					VertexCompatAPIKey: []config.VertexCompatKey{
						{APIKey: "vc-" + suffix, BaseURL: "https://vc-" + suffix + ".example.com"},
					},
					CodexKey: []config.CodexKey{
						{APIKey: "cx-" + suffix, BaseURL: "https://cx-" + suffix + ".example.com"},
					},
					OAuthExcludedModels: map[string][]string{
						"claude": {"claude-" + suffix + "-model"},
					},
					SDKConfig: config.SDKConfig{ForceModelPrefix: flip},
					OpenAICompatibility: []config.OpenAICompatibility{{
						Name: "compat", Models: []config.OpenAICompatibilityModel{{Name: "model-" + suffix}},
					}},
				}
				s.cfgMu.Lock()
				s.cfg = newCfg
				s.cfgMu.Unlock()
			}
		}
	}()

	time.Sleep(80 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestBuilderClonesRuntimeConfig(t *testing.T) {
	source := &config.Config{
		AuthDir:   t.TempDir(),
		SDKConfig: config.SDKConfig{ForceModelPrefix: true},
		ClaudeKey: []config.ClaudeKey{{APIKey: "source-key"}},
	}
	service, err := NewBuilder().WithConfig(source).WithConfigPath(t.TempDir() + "/config.yaml").Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	source.ForceModelPrefix = false
	source.ClaudeKey[0].APIKey = "mutated-key"
	snapshot := service.configSnapshot()
	if snapshot == source {
		t.Fatal("runtime config shares the caller's pointer")
	}
	if !snapshot.ForceModelPrefix || snapshot.ClaudeKey[0].APIKey != "source-key" {
		t.Fatalf("runtime config changed with caller mutation: %+v", snapshot)
	}
}
