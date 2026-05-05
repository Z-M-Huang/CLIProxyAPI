package management

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/redisqueue"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usagestore"
)

type usageExportPayload struct {
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	Usage      any       `json:"usage"`
}

type usageImportPayload struct {
	Version int             `json:"version"`
	Usage   json.RawMessage `json:"usage"`
}

// GetUsageStatistics returns the request statistics snapshot.
func (h *Handler) GetUsageStatistics(c *gin.Context) {
	if h != nil && h.usageStore != nil {
		snapshot, err := h.usageStore.BuildSnapshot(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to load usage statistics: %v", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"usage":           snapshot,
			"failed_requests": snapshot.FailureCount,
		})
		return
	}

	var snapshot usage.StatisticsSnapshot
	if h != nil && h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, gin.H{
		"usage":           snapshot,
		"failed_requests": snapshot.FailureCount,
	})
}

// ExportUsageStatistics returns a complete usage snapshot for backup/migration.
func (h *Handler) ExportUsageStatistics(c *gin.Context) {
	if h != nil && h.usageStore != nil {
		snapshot, err := h.usageStore.BuildSnapshot(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to export usage statistics: %v", err)})
			return
		}
		c.JSON(http.StatusOK, usageExportPayload{
			Version:    1,
			ExportedAt: time.Now().UTC(),
			Usage:      snapshot,
		})
		return
	}

	var snapshot usage.StatisticsSnapshot
	if h != nil && h.usageStats != nil {
		snapshot = h.usageStats.Snapshot()
	}
	c.JSON(http.StatusOK, usageExportPayload{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Usage:      snapshot,
	})
}

// ImportUsageStatistics merges a previously exported usage snapshot.
func (h *Handler) ImportUsageStatistics(c *gin.Context) {
	if h == nil || (h.usageStats == nil && h.usageStore == nil) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "usage statistics unavailable"})
		return
	}

	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var payload usageImportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if payload.Version != 0 && payload.Version != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported version"})
		return
	}
	rawUsage := payload.Usage
	if len(rawUsage) == 0 || string(rawUsage) == "null" {
		rawUsage = data
	}

	if h.usageStore != nil {
		var snapshot usagestore.StatisticsSnapshot
		if err := json.Unmarshal(rawUsage, &snapshot); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid usage snapshot"})
			return
		}
		result, err := h.usageStore.ImportSnapshot(c.Request.Context(), snapshot)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to import usage statistics: %v", err)})
			return
		}
		current, err := h.usageStore.BuildSnapshot(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to load usage statistics: %v", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"added":           result.Added,
			"skipped":         result.Skipped,
			"total_requests":  current.TotalRequests,
			"failed_requests": current.FailureCount,
		})
		return
	}

	var snapshot usage.StatisticsSnapshot
	if err := json.Unmarshal(rawUsage, &snapshot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid usage snapshot"})
		return
	}
	result := h.usageStats.MergeSnapshot(snapshot)
	current := h.usageStats.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"added":           result.Added,
		"skipped":         result.Skipped,
		"total_requests":  current.TotalRequests,
		"failed_requests": current.FailureCount,
	})
}

// GetUsageOverview returns aggregate usage metrics. It currently shares the
// snapshot shape with /usage so existing dashboard code can consume it.
func (h *Handler) GetUsageOverview(c *gin.Context) {
	h.GetUsageStatistics(c)
}

// GetUsageEvents returns persisted usage events with optional filtering.
func (h *Handler) GetUsageEvents(c *gin.Context) {
	filter, err := parseUsageEventsFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h == nil || h.usageStore == nil {
		page := filter.Page
		if page <= 0 {
			page = 1
		}
		pageSize := filter.PageSize
		if pageSize <= 0 {
			pageSize = 100
		}
		c.JSON(http.StatusOK, usagestore.UsageEventsPage{
			Events:     []usagestore.UsageEventRecord{},
			Models:     []string{},
			Sources:    []string{},
			Page:       page,
			PageSize:   pageSize,
			TotalPages: 0,
		})
		return
	}
	page, err := h.usageStore.ListUsageEvents(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list usage events: %v", err)})
		return
	}
	c.JSON(http.StatusOK, page)
}

// GetUsageEventFilters returns model/source facets for persisted usage events.
func (h *Handler) GetUsageEventFilters(c *gin.Context) {
	filter, err := parseUsageEventsFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	filter.Page = 1
	filter.PageSize = 1
	if h == nil || h.usageStore == nil {
		c.JSON(http.StatusOK, gin.H{"models": []string{}, "sources": []string{}})
		return
	}
	page, err := h.usageStore.ListUsageEvents(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list usage event filters: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": page.Models, "sources": page.Sources})
}

func parseUsageEventsFilter(c *gin.Context) (usagestore.UsageEventsFilter, error) {
	filter := usagestore.UsageEventsFilter{
		Model:  strings.TrimSpace(c.Query("model")),
		Source: strings.TrimSpace(c.Query("source")),
		Result: strings.ToLower(strings.TrimSpace(c.Query("result"))),
	}
	page, err := parseOptionalPositiveInt(c.Query("page"), 1, 0)
	if err != nil {
		return filter, fmt.Errorf("page must be a positive integer")
	}
	pageSize, err := parseOptionalPositiveInt(firstUsageQuery(c, "page_size", "pageSize", "limit"), 100, 1000)
	if err != nil {
		return filter, fmt.Errorf("page_size must be a positive integer")
	}
	filter.Page = page
	filter.PageSize = pageSize
	if filter.Result != "" && filter.Result != "success" && filter.Result != "failed" {
		return filter, fmt.Errorf("result must be success or failed")
	}
	start, err := parseOptionalUsageTime(firstUsageQuery(c, "start_time", "start", "from"))
	if err != nil {
		return filter, fmt.Errorf("invalid start_time")
	}
	end, err := parseOptionalUsageTime(firstUsageQuery(c, "end_time", "end", "to"))
	if err != nil {
		return filter, fmt.Errorf("invalid end_time")
	}
	filter.StartTime = start
	filter.EndTime = end
	return filter, nil
}

func firstUsageQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}

func parseOptionalPositiveInt(value string, fallback int, max int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, errors.New("must be a positive integer")
	}
	if max > 0 && n > max {
		return max, nil
	}
	return n, nil
}

func parseOptionalUsageTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		parsed := t.UTC()
		return &parsed, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		parsed := t.UTC()
		return &parsed, nil
	}
	unixValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, err
	}
	if unixValue > 10_000_000_000 {
		parsed := time.UnixMilli(unixValue).UTC()
		return &parsed, nil
	}
	parsed := time.Unix(unixValue, 0).UTC()
	return &parsed, nil
}

type usageQueueRecord []byte

func (r usageQueueRecord) MarshalJSON() ([]byte, error) {
	if json.Valid(r) {
		return append([]byte(nil), r...), nil
	}
	return json.Marshal(string(r))
}

// GetUsageQueue pops queued usage records from the usage queue.
func (h *Handler) GetUsageQueue(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}

	count, errCount := parseUsageQueueCount(c.Query("count"))
	if errCount != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errCount.Error()})
		return
	}

	items := redisqueue.PopOldest(count)
	records := make([]usageQueueRecord, 0, len(items))
	for _, item := range items {
		records = append(records, usageQueueRecord(append([]byte(nil), item...)))
	}

	c.JSON(http.StatusOK, records)
}

func parseUsageQueueCount(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 1, nil
	}
	count, errCount := strconv.Atoi(value)
	if errCount != nil || count <= 0 {
		return 0, errors.New("count must be a positive integer")
	}
	return count, nil
}
