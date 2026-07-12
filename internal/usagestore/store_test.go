package usagestore

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistsUsageEventsAndRequestHistory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "usage.sqlite")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	timestamp := time.Date(2026, 5, 5, 12, 30, 0, 0, time.UTC)
	event := UsageEvent{
		APIGroupKey: "test-key",
		Provider:    "claude",
		Endpoint:    "/v1/messages",
		RequestID:   "req-1",
		Model:       "claude-sonnet-4",
		Timestamp:   timestamp,
		Source:      "account@example.com",
		AuthIndex:   "claude-1",
		LatencyMS:   250,
		Tokens: TokenStats{
			InputTokens:         10,
			OutputTokens:        20,
			CacheReadTokens:     3,
			CacheCreationTokens: 4,
		},
	}
	inserted, err := store.InsertUsageEvent(ctx, event)
	if err != nil {
		t.Fatalf("InsertUsageEvent() error = %v", err)
	}
	if !inserted {
		t.Fatal("InsertUsageEvent() inserted = false, want true")
	}
	assertMigrationVersion(t, store, 3)

	inserted, err = store.InsertUsageEvent(ctx, event)
	if err != nil {
		t.Fatalf("duplicate InsertUsageEvent() error = %v", err)
	}
	if inserted {
		t.Fatal("duplicate InsertUsageEvent() inserted = true, want false")
	}

	if err := store.InsertRequestHistory(ctx, RequestHistory{
		RequestID:        "req-1",
		LogName:          "error-test-req-1.log",
		URL:              "/v1/messages",
		Method:           "POST",
		StatusCode:       500,
		Force:            true,
		RequestTimestamp: timestamp,
		LogText:          []byte("request history body"),
	}); err != nil {
		t.Fatalf("InsertRequestHistory() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	store, err = Open(dbPath)
	if err != nil {
		t.Fatalf("reopen Open() error = %v", err)
	}
	assertMigrationVersion(t, store, 3)
	defer func() {
		if errClose := store.Close(); errClose != nil {
			t.Fatalf("Close() error = %v", errClose)
		}
	}()

	snapshot, err := store.BuildSnapshot(ctx)
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	if snapshot.TotalRequests != 1 || snapshot.SuccessCount != 1 || snapshot.TotalTokens != 30 {
		t.Fatalf("BuildSnapshot() totals = requests:%d success:%d tokens:%d, want 1/1/30", snapshot.TotalRequests, snapshot.SuccessCount, snapshot.TotalTokens)
	}
	apiSnapshot, ok := snapshot.APIs["test-key"]
	if !ok {
		t.Fatalf("BuildSnapshot() missing API key")
	}
	modelSnapshot, ok := apiSnapshot.Models["claude-sonnet-4"]
	if !ok {
		t.Fatalf("BuildSnapshot() missing model")
	}
	if len(modelSnapshot.Details) != 1 || modelSnapshot.Details[0].RequestID != "req-1" {
		t.Fatalf("BuildSnapshot() details = %#v, want req-1 detail", modelSnapshot.Details)
	}
	if modelSnapshot.Details[0].Tokens.CacheReadTokens != 3 || modelSnapshot.Details[0].Tokens.CacheCreationTokens != 4 {
		t.Fatalf("BuildSnapshot() cache tokens = %+v, want read=3 creation=4", modelSnapshot.Details[0].Tokens)
	}

	page, err := store.ListUsageEvents(ctx, UsageEventsFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListUsageEvents() error = %v", err)
	}
	if page.TotalCount != 1 || len(page.Events) != 1 {
		t.Fatalf("ListUsageEvents() total=%d len=%d, want 1/1", page.TotalCount, len(page.Events))
	}
	if page.Events[0].Tokens.CacheReadTokens != 3 || page.Events[0].Tokens.CacheCreationTokens != 4 {
		t.Fatalf("ListUsageEvents() cache tokens = %+v, want read=3 creation=4", page.Events[0].Tokens)
	}

	history, err := store.RequestHistoryByID(ctx, "req-1")
	if err != nil {
		t.Fatalf("RequestHistoryByID() error = %v", err)
	}
	if string(history.LogText) != "request history body" {
		t.Fatalf("RequestHistoryByID() LogText = %q, want request history body", history.LogText)
	}
	files, err := store.ListErrorRequestHistories(ctx)
	if err != nil {
		t.Fatalf("ListErrorRequestHistories() error = %v", err)
	}
	if len(files) != 1 || files[0].Name != "error-test-req-1.log" {
		t.Fatalf("ListErrorRequestHistories() = %#v, want error-test-req-1.log", files)
	}
}

func TestStoreOverviewRollupsSurviveRawEventRetention(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if errClose := store.Close(); errClose != nil {
			t.Fatalf("Close() error = %v", errClose)
		}
	}()

	baseTime := time.Date(2026, 6, 1, 12, 4, 0, 0, time.UTC)
	events := []UsageEvent{
		{EventKey: "event-1", APIGroupKey: "team", Provider: "claude", Model: "sonnet", Timestamp: baseTime, Source: "account", AuthIndex: "auth-1", LatencyMS: 100, Tokens: TokenStats{InputTokens: 10, OutputTokens: 5}},
		{EventKey: "event-2", APIGroupKey: "team", Provider: "claude", Model: "sonnet", Timestamp: baseTime.Add(5 * time.Minute), Source: "account", AuthIndex: "auth-1", LatencyMS: 300, Tokens: TokenStats{InputTokens: 20, OutputTokens: 10}},
		{EventKey: "event-3", APIGroupKey: "team", Provider: "claude", Model: "sonnet", Timestamp: baseTime.Add(7 * time.Minute), Source: "account", AuthIndex: "auth-1", Failed: true, Tokens: TokenStats{InputTokens: 3}},
	}
	for _, event := range events {
		inserted, errInsert := store.InsertUsageEvent(ctx, event)
		if errInsert != nil || !inserted {
			t.Fatalf("InsertUsageEvent(%s) = inserted:%t err:%v", event.EventKey, inserted, errInsert)
		}
	}

	overview, err := store.BuildOverview(ctx)
	if err != nil {
		t.Fatalf("BuildOverview() error = %v", err)
	}
	if overview.TotalRequests != 3 || overview.SuccessCount != 2 || overview.FailureCount != 1 || overview.TotalTokens != 48 {
		t.Fatalf("overview totals = requests:%d success:%d failure:%d tokens:%d", overview.TotalRequests, overview.SuccessCount, overview.FailureCount, overview.TotalTokens)
	}
	model := overview.APIs["team"].Models["sonnet"]
	if model.TotalRequests != 3 || model.SuccessCount != 2 || model.FailureCount != 1 || len(model.Details) != 2 {
		t.Fatalf("overview model = %+v", model)
	}
	var successful RequestDetail
	for _, detail := range model.Details {
		if !detail.Failed {
			successful = detail
		}
	}
	if successful.RequestCount != 2 || successful.LatencyTotalMS != 400 || successful.LatencySampleCount != 2 || successful.LatencyMS != 200 {
		t.Fatalf("successful rollup = %+v", successful)
	}

	page, err := store.ListUsageEvents(ctx, UsageEventsFilter{Page: 1, PageSize: 10, AuthIndex: "auth-1"})
	if err != nil {
		t.Fatalf("ListUsageEvents() error = %v", err)
	}
	if page.TotalCount != 3 || len(page.AuthIndexes) != 1 || page.AuthIndexes[0] != "auth-1" {
		t.Fatalf("usage page = %+v", page)
	}

	deleted, err := store.PruneUsageEventsBefore(ctx, baseTime.Add(24*time.Hour))
	if err != nil || deleted != 3 {
		t.Fatalf("PruneUsageEventsBefore() = deleted:%d err:%v", deleted, err)
	}
	page, err = store.ListUsageEvents(ctx, UsageEventsFilter{Page: 1, PageSize: 10})
	if err != nil || page.TotalCount != 0 {
		t.Fatalf("retained raw events = %d err:%v", page.TotalCount, err)
	}
	overviewAfterPrune, err := store.BuildOverview(ctx)
	if err != nil || overviewAfterPrune.TotalRequests != 3 || overviewAfterPrune.TotalTokens != 48 {
		t.Fatalf("overview after prune = %+v err:%v", overviewAfterPrune, err)
	}
	inserted, err := store.InsertUsageEvent(ctx, events[0])
	if err != nil || inserted {
		t.Fatalf("retained event key dedup = inserted:%t err:%v", inserted, err)
	}
}

func TestStoreMigrationBackfillsExistingUsageEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "usage.sqlite")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err = store.db.Exec("DELETE FROM " + migrationTableName + " WHERE version_id = 3"); err != nil {
		t.Fatalf("remove v3 migration record: %v", err)
	}
	if _, err = store.db.Exec("DROP TABLE usage_rollups"); err != nil {
		t.Fatalf("drop usage_rollups: %v", err)
	}
	if _, err = store.db.Exec("DROP TABLE usage_event_keys"); err != nil {
		t.Fatalf("drop usage_event_keys: %v", err)
	}
	if _, err = store.db.Exec(`INSERT INTO usage_events (
		event_key, api_group_key, provider, endpoint, auth_type, request_id, model, timestamp,
		source, auth_index, failed, latency_ms, input_tokens, output_tokens, reasoning_tokens,
		cached_tokens, cache_read_tokens, cache_creation_tokens, total_tokens, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy-event", "legacy-api", "claude", "/v1/messages", "oauth", "req-legacy", "sonnet",
		"2026-05-01T12:07:00Z", "account", "auth-legacy", 0, 250, 10, 20, 3, 4, 5, 6, 33,
		"2026-05-01T12:07:01Z",
	); err != nil {
		t.Fatalf("insert legacy usage event: %v", err)
	}
	if err = store.Close(); err != nil {
		t.Fatalf("close v2 store: %v", err)
	}

	store, err = Open(dbPath)
	if err != nil {
		t.Fatalf("reopen migrated store: %v", err)
	}
	defer func() {
		if errClose := store.Close(); errClose != nil {
			t.Fatalf("Close() error = %v", errClose)
		}
	}()
	assertMigrationVersion(t, store, 3)

	overview, err := store.BuildOverview(ctx)
	if err != nil {
		t.Fatalf("BuildOverview() error = %v", err)
	}
	model := overview.APIs["legacy-api"].Models["sonnet"]
	if overview.TotalRequests != 1 || overview.TotalTokens != 33 || model.TotalRequests != 1 || len(model.Details) != 1 {
		t.Fatalf("migrated overview = %+v", overview)
	}
	detail := model.Details[0]
	if detail.Timestamp != time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) || detail.LatencyTotalMS != 250 || detail.LatencySampleCount != 1 {
		t.Fatalf("migrated detail = %+v", detail)
	}

	inserted, err := store.InsertUsageEvent(ctx, UsageEvent{EventKey: "legacy-event"})
	if err != nil || inserted {
		t.Fatalf("legacy dedup key = inserted:%t err:%v", inserted, err)
	}
}

func assertMigrationVersion(t *testing.T, store *Store, want int64) {
	t.Helper()

	var version int64
	if err := store.db.QueryRow("SELECT MAX(version_id) FROM " + migrationTableName).Scan(&version); err != nil {
		t.Fatalf("migration version query error = %v", err)
	}
	if version != want {
		t.Fatalf("migration version = %d, want %d", version, want)
	}
}
