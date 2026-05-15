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
	assertMigrationVersion(t, store, 2)

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
	assertMigrationVersion(t, store, 2)
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
