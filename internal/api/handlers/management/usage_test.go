package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/redisqueue"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/usagestore"
)

func TestPersistedUsageOverviewAndEventFilters(t *testing.T) {
	t.Parallel()

	store, err := usagestore.Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if errClose := store.Close(); errClose != nil {
			t.Fatalf("Close() error = %v", errClose)
		}
	}()

	baseTime := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	for _, event := range []usagestore.UsageEvent{
		{EventKey: "event-a", APIGroupKey: "team", Model: "model-a", Timestamp: baseTime, AuthIndex: "auth-a", Tokens: usagestore.TokenStats{TotalTokens: 10}},
		{EventKey: "event-b", APIGroupKey: "team", Model: "model-a", Timestamp: baseTime.Add(time.Minute), AuthIndex: "auth-b", Failed: true, Tokens: usagestore.TokenStats{TotalTokens: 20}},
	} {
		inserted, errInsert := store.InsertUsageEvent(context.Background(), event)
		if errInsert != nil || !inserted {
			t.Fatalf("InsertUsageEvent(%s) = inserted:%t err:%v", event.EventKey, inserted, errInsert)
		}
	}
	h := &Handler{usageStore: store}

	overviewRecorder := httptest.NewRecorder()
	overviewContext, _ := gin.CreateTestContext(overviewRecorder)
	overviewContext.Request = httptest.NewRequest(http.MethodGet, "/v0/management/usage/overview", nil)
	h.GetUsageOverview(overviewContext)
	if overviewRecorder.Code != http.StatusOK {
		t.Fatalf("overview status = %d body=%s", overviewRecorder.Code, overviewRecorder.Body.String())
	}
	var overviewPayload struct {
		Usage usagestore.StatisticsSnapshot `json:"usage"`
	}
	if err = json.Unmarshal(overviewRecorder.Body.Bytes(), &overviewPayload); err != nil {
		t.Fatalf("unmarshal overview: %v", err)
	}
	if overviewPayload.Usage.TotalRequests != 2 || overviewPayload.Usage.SuccessCount != 1 || overviewPayload.Usage.FailureCount != 1 {
		t.Fatalf("overview usage = %+v", overviewPayload.Usage)
	}

	eventsRecorder := httptest.NewRecorder()
	eventsContext, _ := gin.CreateTestContext(eventsRecorder)
	eventsContext.Request = httptest.NewRequest(http.MethodGet, "/v0/management/usage/events?auth_index=auth-b", nil)
	h.GetUsageEvents(eventsContext)
	if eventsRecorder.Code != http.StatusOK {
		t.Fatalf("events status = %d body=%s", eventsRecorder.Code, eventsRecorder.Body.String())
	}
	var page usagestore.UsageEventsPage
	if err = json.Unmarshal(eventsRecorder.Body.Bytes(), &page); err != nil {
		t.Fatalf("unmarshal events: %v", err)
	}
	if page.TotalCount != 1 || len(page.Events) != 1 || page.Events[0].AuthIndex != "auth-b" {
		t.Fatalf("filtered events = %+v", page)
	}
}

func TestGetUsageQueuePopsRequestedRecords(t *testing.T) {
	withManagementUsageQueue(t, func() {
		redisqueue.Enqueue([]byte(`{"id":1}`))
		redisqueue.Enqueue([]byte(`{"id":2}`))
		redisqueue.Enqueue([]byte(`{"id":3}`))

		rec := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(rec)
		ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/usage-queue?count=2", nil)

		h := &Handler{}
		h.GetUsageQueue(ginCtx)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload []json.RawMessage
		if errUnmarshal := json.Unmarshal(rec.Body.Bytes(), &payload); errUnmarshal != nil {
			t.Fatalf("unmarshal response: %v", errUnmarshal)
		}
		if len(payload) != 2 {
			t.Fatalf("response records = %d, want 2", len(payload))
		}
		requireRecordID(t, payload[0], 1)
		requireRecordID(t, payload[1], 2)

		remaining := redisqueue.PopOldest(10)
		if len(remaining) != 1 || string(remaining[0]) != `{"id":3}` {
			t.Fatalf("remaining queue = %q, want third item only", remaining)
		}
	})
}

func TestGetUsageQueueInvalidCountDoesNotPop(t *testing.T) {
	withManagementUsageQueue(t, func() {
		redisqueue.Enqueue([]byte(`{"id":1}`))

		rec := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(rec)
		ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/usage-queue?count=0", nil)

		h := &Handler{}
		h.GetUsageQueue(ginCtx)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}

		remaining := redisqueue.PopOldest(10)
		if len(remaining) != 1 || string(remaining[0]) != `{"id":1}` {
			t.Fatalf("remaining queue = %q, want original item", remaining)
		}
	})
}

func withManagementUsageQueue(t *testing.T, fn func()) {
	t.Helper()

	prevQueueEnabled := redisqueue.Enabled()
	redisqueue.SetEnabled(false)
	redisqueue.SetEnabled(true)

	defer func() {
		redisqueue.SetEnabled(false)
		redisqueue.SetEnabled(prevQueueEnabled)
	}()

	fn()
}

func requireRecordID(t *testing.T, raw json.RawMessage, want int) {
	t.Helper()

	var payload struct {
		ID int `json:"id"`
	}
	if errUnmarshal := json.Unmarshal(raw, &payload); errUnmarshal != nil {
		t.Fatalf("unmarshal record: %v", errUnmarshal)
	}
	if payload.ID != want {
		t.Fatalf("record id = %d, want %d", payload.ID, want)
	}
}
