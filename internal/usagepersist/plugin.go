package usagepersist

import (
	"context"
	"strings"
	"sync"
	"time"

	internallogging "github.com/router-for-me/CLIProxyAPI/v7/internal/logging"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usagestore"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
	log "github.com/sirupsen/logrus"
)

var (
	storeMu sync.RWMutex
	store   *usagestore.Store
	once    sync.Once
)

func SetStore(next *usagestore.Store) {
	once.Do(func() {
		coreusage.RegisterPlugin(&plugin{})
	})
	storeMu.Lock()
	store = next
	storeMu.Unlock()
}

type plugin struct{}

func (p *plugin) HandleUsage(ctx context.Context, record coreusage.Record) {
	if p == nil || !usage.StatisticsEnabled() {
		return
	}
	current := currentStore()
	if current == nil {
		return
	}

	timestamp := record.RequestedAt
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	failed := record.Failed
	if !failed {
		failed = !resolveSuccess(ctx)
	}

	event := usagestore.NormalizeUsageEvent(usagestore.UsageEvent{
		APIGroupKey: firstNonEmpty(record.APIKey, record.Provider, resolveEndpoint(ctx), "unknown"),
		Provider:    record.Provider,
		Endpoint:    resolveEndpoint(ctx),
		AuthType:    record.AuthType,
		RequestID:   strings.TrimSpace(internallogging.GetRequestID(ctx)),
		Model:       record.Model,
		Timestamp:   timestamp,
		Source:      record.Source,
		AuthIndex:   record.AuthIndex,
		Failed:      failed,
		LatencyMS:   normaliseLatency(record.Latency),
		Tokens: usagestore.TokenStats{
			InputTokens:     record.Detail.InputTokens,
			OutputTokens:    record.Detail.OutputTokens,
			ReasoningTokens: record.Detail.ReasoningTokens,
			CachedTokens:    record.Detail.CachedTokens,
			TotalTokens:     record.Detail.TotalTokens,
		},
	})
	if _, err := current.InsertUsageEvent(context.Background(), event); err != nil {
		log.WithError(err).Warn("failed to persist usage event")
	}
}

func currentStore() *usagestore.Store {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return store
}

func resolveSuccess(ctx context.Context) bool {
	status := internallogging.GetResponseStatus(ctx)
	if status == 0 {
		return true
	}
	return status < 400
}

func resolveEndpoint(ctx context.Context) string {
	return strings.TrimSpace(internallogging.GetEndpoint(ctx))
}

func normaliseLatency(latency time.Duration) int64 {
	if latency <= 0 {
		return 0
	}
	return latency.Milliseconds()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
