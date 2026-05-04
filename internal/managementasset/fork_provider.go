package managementasset

const (
	forkManagementReleaseURL  = "https://api.github.com/repos/Z-M-Huang/Cli-Proxy-API-Management-Center/releases/latest"
	forkManagementFallbackURL = "https://github.com/Z-M-Huang/Cli-Proxy-API-Management-Center/releases/latest/download/management.html"
)

type forkReleaseURLProvider struct{}

func (forkReleaseURLProvider) ReleaseURL() string  { return forkManagementReleaseURL }
func (forkReleaseURLProvider) FallbackURL() string { return forkManagementFallbackURL }

// UseForkReleaseURLProvider installs the fork-specific management asset release
// endpoints without modifying the upstream-default seam in updater.go.
func UseForkReleaseURLProvider() {
	SetReleaseURLProvider(forkReleaseURLProvider{})
}
