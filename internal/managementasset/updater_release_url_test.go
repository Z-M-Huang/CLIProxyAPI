package managementasset

import (
	"testing"
)

// TestDefaultReleaseURLProvider_MatchesUpstreamConstants pins the contract
// that the seam's default impl returns the upstream URLs byte-for-byte. If a
// future change swaps the upstream URL, this test surfaces it as part of an
// upstream-PRable change rather than a silent fork divergence.
func TestDefaultReleaseURLProvider_MatchesUpstreamConstants(t *testing.T) {
	p := defaultReleaseURLProvider{}
	if got, want := p.ReleaseURL(), defaultManagementReleaseURL; got != want {
		t.Fatalf("default ReleaseURL = %q, want %q", got, want)
	}
	if got, want := p.FallbackURL(), defaultManagementFallbackURL; got != want {
		t.Fatalf("default FallbackURL = %q, want %q", got, want)
	}
}

// TestSetReleaseURLProvider_OverridesAndRestores verifies that
// SetReleaseURLProvider swaps the active provider, that the seam accessors
// see the swap, and that passing nil restores the default impl.
func TestSetReleaseURLProvider_OverridesAndRestores(t *testing.T) {
	t.Cleanup(func() { SetReleaseURLProvider(nil) })

	custom := stubReleaseURLProvider{
		release:  "https://example.test/repos/me/myfork/releases/latest",
		fallback: "https://example.test/cpamc/",
	}
	SetReleaseURLProvider(custom)

	if got, want := currentReleaseURL(), custom.release; got != want {
		t.Fatalf("currentReleaseURL after override = %q, want %q", got, want)
	}
	if got, want := currentFallbackURL(), custom.fallback; got != want {
		t.Fatalf("currentFallbackURL after override = %q, want %q", got, want)
	}

	// nil restores defaults.
	SetReleaseURLProvider(nil)
	if got, want := currentReleaseURL(), defaultManagementReleaseURL; got != want {
		t.Fatalf("currentReleaseURL after restore = %q, want %q", got, want)
	}
	if got, want := currentFallbackURL(), defaultManagementFallbackURL; got != want {
		t.Fatalf("currentFallbackURL after restore = %q, want %q", got, want)
	}
}

// TestCurrentReleaseURL_FallsBackOnEmpty ensures a provider that returns the
// empty string still yields a usable URL — important so an incomplete fork
// impl doesn't silently break the auto-updater.
func TestCurrentReleaseURL_FallsBackOnEmpty(t *testing.T) {
	t.Cleanup(func() { SetReleaseURLProvider(nil) })

	SetReleaseURLProvider(stubReleaseURLProvider{release: "", fallback: ""})

	if got, want := currentReleaseURL(), defaultManagementReleaseURL; got != want {
		t.Fatalf("empty provider ReleaseURL fallback = %q, want %q", got, want)
	}
	if got, want := currentFallbackURL(), defaultManagementFallbackURL; got != want {
		t.Fatalf("empty provider FallbackURL fallback = %q, want %q", got, want)
	}
}

// TestResolveReleaseURL_UsesProviderDefault validates that when the
// per-config repository override is empty, resolveReleaseURL routes through
// the active provider rather than the constant.
func TestResolveReleaseURL_UsesProviderDefault(t *testing.T) {
	t.Cleanup(func() { SetReleaseURLProvider(nil) })

	custom := stubReleaseURLProvider{
		release: "https://example.test/repos/forkowner/forkrepo/releases/latest",
	}
	SetReleaseURLProvider(custom)

	if got, want := resolveReleaseURL(""), custom.release; got != want {
		t.Fatalf("resolveReleaseURL(\"\") = %q, want provider default %q", got, want)
	}
	// A per-config repository should still win over the provider default.
	if got, want := resolveReleaseURL("https://github.com/other/explicit"), "https://api.github.com/repos/other/explicit/releases/latest"; got != want {
		t.Fatalf("resolveReleaseURL(github URL) = %q, want %q", got, want)
	}
}

func TestResolveReleaseURL_AcceptsExplicitReleaseURLs(t *testing.T) {
	t.Cleanup(func() { SetReleaseURLProvider(nil) })

	tests := []struct {
		name string
		repo string
		want string
	}{
		{
			name: "api tag release",
			repo: "https://api.github.com/repos/Z-M-Huang/Cli-Proxy-API-Management-Center/releases/tags/zmh-v1.2.0-rc.0",
			want: "https://api.github.com/repos/Z-M-Huang/Cli-Proxy-API-Management-Center/releases/tags/zmh-v1.2.0-rc.0",
		},
		{
			name: "github tag release",
			repo: "https://github.com/Z-M-Huang/Cli-Proxy-API-Management-Center/releases/tag/zmh-v1.2.0-rc.0",
			want: "https://api.github.com/repos/Z-M-Huang/Cli-Proxy-API-Management-Center/releases/tags/zmh-v1.2.0-rc.0",
		},
		{
			name: "api releases collection",
			repo: "https://api.github.com/repos/Z-M-Huang/Cli-Proxy-API-Management-Center/releases",
			want: "https://api.github.com/repos/Z-M-Huang/Cli-Proxy-API-Management-Center/releases/latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveReleaseURL(tt.repo); got != tt.want {
				t.Fatalf("resolveReleaseURL(%q) = %q, want %q", tt.repo, got, tt.want)
			}
		})
	}
}

func TestResolveReleaseURL_EnvOverrideWins(t *testing.T) {
	t.Cleanup(func() { SetReleaseURLProvider(nil) })

	override := "https://api.github.com/repos/Z-M-Huang/Cli-Proxy-API-Management-Center/releases/tags/zmh-v1.2.0-rc.0"
	t.Setenv(managementPanelReleaseURLEnv, override)

	if got := resolveReleaseURL("https://github.com/other/explicit"); got != override {
		t.Fatalf("resolveReleaseURL() with %s = %q, want %q", managementPanelReleaseURLEnv, got, override)
	}
}

type stubReleaseURLProvider struct {
	release  string
	fallback string
}

func (s stubReleaseURLProvider) ReleaseURL() string  { return s.release }
func (s stubReleaseURLProvider) FallbackURL() string { return s.fallback }
