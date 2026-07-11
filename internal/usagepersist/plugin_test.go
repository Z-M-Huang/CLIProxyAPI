package usagepersist

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	internalusage "github.com/router-for-me/CLIProxyAPI/v7/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usagestore"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestPluginPersistsCompleteCacheTokenBreakdown(t *testing.T) {
	store, err := usagestore.Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	statisticsEnabled := internalusage.StatisticsEnabled()
	internalusage.SetStatisticsEnabled(true)
	SetStore(store)
	defer func() {
		SetStore(nil)
		internalusage.SetStatisticsEnabled(statisticsEnabled)
		if errClose := store.Close(); errClose != nil {
			t.Fatalf("Close() error = %v", errClose)
		}
	}()

	(&plugin{}).HandleUsage(context.Background(), coreusage.Record{
		Provider:    "claude",
		Model:       "sonnet",
		RequestedAt: time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
		Detail: coreusage.Detail{
			InputTokens:         10,
			OutputTokens:        20,
			CachedTokens:        3,
			CacheReadTokens:     4,
			CacheCreationTokens: 5,
			TotalTokens:         30,
		},
	})

	page, err := store.ListUsageEvents(context.Background(), usagestore.UsageEventsFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListUsageEvents() error = %v", err)
	}
	if page.TotalCount != 1 || len(page.Events) != 1 {
		t.Fatalf("persisted events = %+v", page)
	}
	tokens := page.Events[0].Tokens
	if tokens.CachedTokens != 3 || tokens.CacheReadTokens != 4 || tokens.CacheCreationTokens != 5 {
		t.Fatalf("persisted cache tokens = %+v", tokens)
	}
}
