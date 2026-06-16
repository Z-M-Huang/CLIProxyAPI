// Package cmd provides command-line interface functionality for the CLI Proxy API server.
// It includes authentication flows for various AI service providers, service startup,
// and other command-line operations.
package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/api"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/pluginhost"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/safemode"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usagepersist"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usagestore"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy"
	log "github.com/sirupsen/logrus"
)

// StartService builds and runs the proxy service using the exported SDK.
// It creates a new proxy service instance, sets up signal handling for graceful shutdown,
// and starts the service with the provided configuration.
//
// Parameters:
//   - cfg: The application configuration
//   - configPath: The path to the configuration file
//   - localPassword: Optional password accepted for local management requests
func StartService(cfg *config.Config, configPath string, localPassword string) {
	StartServiceWithPluginHost(cfg, configPath, localPassword, nil)
}

// StartServiceWithPluginHost builds and runs the proxy service with a shared plugin host.
func StartServiceWithPluginHost(cfg *config.Config, configPath string, localPassword string, host *pluginhost.Host) {
	store, errStore := openUsageStore(cfg, configPath)
	if errStore != nil {
		log.Errorf("failed to open usage database: %v", errStore)
		return
	}
	usagepersist.SetStore(store)
	defer closeUsageStore(store)

	builder := cliproxy.NewBuilder().
		WithConfig(cfg).
		WithConfigPath(configPath).
		WithLocalManagementPassword(localPassword).
		WithServerOptions(usageServerOptions(store)...)
	if host != nil {
		builder = builder.WithPluginHost(host)
	}

	ctxSignal, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	runCtx := ctxSignal
	if localPassword != "" {
		var keepAliveCancel context.CancelFunc
		runCtx, keepAliveCancel = context.WithCancel(ctxSignal)
		builder = builder.WithServerOptions(api.WithKeepAliveEndpoint(10*time.Second, func() {
			log.Warn("keep-alive endpoint idle for 10s, shutting down")
			keepAliveCancel()
		}))
	}

	service, err := builder.Build()
	if err != nil {
		log.Errorf("failed to build proxy service: %v", err)
		return
	}

	err = service.Run(runCtx)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Errorf("proxy service exited with error: %v", err)
	}
}

// StartExampleAPIKeyWarningServer starts a warning-only server for unsafe template API keys.
func StartExampleAPIKeyWarningServer(cfg *config.Config, configPath string, keys []string) {
	ctxSignal, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Errorf("normal API server disabled: example API key values are configured in %s", configPath)
	log.Errorf("example API key warning page listening on: %s", safemode.WarningServerURL(cfg))
	if err := safemode.StartExampleAPIKeyWarningServer(ctxSignal, cfg, configPath, keys); err != nil && !errors.Is(err, context.Canceled) {
		log.Errorf("example API key warning server exited with error: %v", err)
	}
}

// StartServiceBackground starts the proxy service in a background goroutine
// and returns a cancel function for shutdown and a done channel.
func StartServiceBackground(cfg *config.Config, configPath string, localPassword string) (cancel func(), done <-chan struct{}) {
	return StartServiceBackgroundWithPluginHost(cfg, configPath, localPassword, nil)
}

// StartServiceBackgroundWithPluginHost starts the proxy service with a shared plugin host.
func StartServiceBackgroundWithPluginHost(cfg *config.Config, configPath string, localPassword string, host *pluginhost.Host) (cancel func(), done <-chan struct{}) {
	store, errStore := openUsageStore(cfg, configPath)
	if errStore != nil {
		log.Errorf("failed to open usage database: %v", errStore)
		doneCh := make(chan struct{})
		close(doneCh)
		return func() {}, doneCh
	}
	usagepersist.SetStore(store)

	builder := cliproxy.NewBuilder().
		WithConfig(cfg).
		WithConfigPath(configPath).
		WithLocalManagementPassword(localPassword).
		WithServerOptions(usageServerOptions(store)...)
	if host != nil {
		builder = builder.WithPluginHost(host)
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	doneCh := make(chan struct{})

	service, err := builder.Build()
	if err != nil {
		log.Errorf("failed to build proxy service: %v", err)
		closeUsageStore(store)
		close(doneCh)
		return cancelFn, doneCh
	}

	go func() {
		defer close(doneCh)
		defer closeUsageStore(store)
		if err := service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Errorf("proxy service exited with error: %v", err)
		}
	}()

	return cancelFn, doneCh
}

// WaitForCloudDeploy waits indefinitely for shutdown signals in cloud deploy mode
// when no configuration file is available.
func WaitForCloudDeploy() {
	// Clarify that we are intentionally idle for configuration and not running the API server.
	log.Info("Cloud deploy mode: No config found; standing by for configuration. API server is not started. Press Ctrl+C to exit.")

	ctxSignal, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Block until shutdown signal is received
	<-ctxSignal.Done()
	log.Info("Cloud deploy mode: Shutdown signal received; exiting")
}

func usageServerOptions(store *usagestore.Store) []api.ServerOption {
	if store == nil {
		return nil
	}
	return []api.ServerOption{
		api.WithUsageStore(store),
		api.WithRequestLoggerFactory(func(cfg *config.Config, _ string) logging.RequestLogger {
			enabled := false
			homeEnabled := false
			if cfg != nil {
				enabled = cfg.RequestLog
				homeEnabled = cfg.Home.Enabled
			}
			logger := logging.NewSQLiteRequestLogger(enabled, store)
			logger.SetHomeEnabled(homeEnabled)
			return logger
		}),
	}
}

func closeUsageStore(store *usagestore.Store) {
	usagepersist.SetStore(nil)
	if store == nil {
		return
	}
	if errClose := store.Close(); errClose != nil {
		log.Warnf("failed to close usage database: %v", errClose)
	}
}

func openUsageStore(cfg *config.Config, configPath string) (*usagestore.Store, error) {
	path := usagestore.DefaultDatabasePath
	if cfg != nil {
		if configured := strings.TrimSpace(cfg.UsageDatabasePath); configured != "" {
			path = configured
		}
	}
	return usagestore.Open(resolveUsageDatabasePath(path, configPath))
}

func resolveUsageDatabasePath(path string, configPath string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = usagestore.DefaultDatabasePath
	}
	if filepath.IsAbs(path) {
		return path
	}
	configDir := strings.TrimSpace(filepath.Dir(configPath))
	if configDir != "" && configDir != "." {
		return filepath.Join(configDir, path)
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		return filepath.Join(wd, path)
	}
	return path
}
